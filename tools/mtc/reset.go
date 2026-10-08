package main

// Comando "mtc reset": reinicia o dispositivo USB da minitela sem alterar firmware nem imagens.
// Substitui o reset-minitela.ps1. Requer Administrador.
// Saidas: 0 ok, 1 falha ao fechar apps ou reiniciar, 2 dispositivo (ou o composto) nao encontrado,
// 3 sem privilegio de administrador, 4 o dispositivo nao voltou em 15 s.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const minitelaHWID = `VID_0324&PID_0324`

// Programas que seguram a porta da minitela (ela aceita um por vez): app oficial da Positivo
// (MiniTelaApp.exe, conforme o AppxManifest) e o Minitela Go.
var holderApps = []string{"MiniTelaApp.exe", "minitela-gui.exe"}

type usbDevice struct {
	InstanceID, Class, Name string
	OK                      bool // iniciado e sem codigo de problema (o "Status OK" do Gerenciador)
}

// minitelaDevices lista o dispositivo composto e as interfaces presentes da minitela.
func minitelaDevices() ([]usbDevice, error) {
	set, err := windows.SetupDiGetClassDevsEx(nil, "", 0, windows.DIGCF_PRESENT|windows.DIGCF_ALLCLASSES, 0, "")
	if err != nil {
		return nil, fmt.Errorf("listar dispositivos: %w", err)
	}
	defer set.Close()

	var devs []usbDevice
	for i := 0; ; i++ {
		data, err := set.EnumDeviceInfo(i)
		if errors.Is(err, windows.ERROR_NO_MORE_ITEMS) {
			return devs, nil
		}
		if err != nil {
			return nil, fmt.Errorf("listar dispositivos: %w", err)
		}
		id, err := set.DeviceInstanceID(data)
		if err != nil || !strings.Contains(strings.ToUpper(id), minitelaHWID) {
			continue
		}
		d := usbDevice{InstanceID: id, Class: devProp(set, data, windows.SPDRP_CLASS)}
		if d.Name = devProp(set, data, windows.SPDRP_FRIENDLYNAME); d.Name == "" {
			d.Name = devProp(set, data, windows.SPDRP_DEVICEDESC)
		}
		var status, problem uint32
		if windows.CM_Get_DevNode_Status(&status, &problem, data.DevInst, 0) == nil {
			d.OK = status&windows.DN_STARTED != 0 && problem == 0
		}
		devs = append(devs, d)
	}
}

func devProp(set windows.DevInfo, data *windows.DevInfoData, p windows.SPDRP) string {
	v, err := set.DeviceRegistryProperty(data, p)
	if err != nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

// parentDevice escolhe o dispositivo composto (sem "&MI_"): reinicia-lo reenumera todas as interfaces.
func parentDevice(devs []usbDevice) (usbDevice, bool) {
	for _, d := range devs {
		if !strings.Contains(strings.ToUpper(d.InstanceID), "&MI_") {
			return d, true
		}
	}
	return usbDevice{}, false
}

// allReady diz se o composto e todas as interfaces voltaram com status OK.
func allReady(devs []usbDevice, parentID string) bool {
	found := false
	for _, d := range devs {
		if !d.OK {
			return false
		}
		found = found || d.InstanceID == parentID
	}
	return found
}

func printDevices(devs []usbDevice) {
	for _, d := range devs {
		status := "OK"
		if !d.OK {
			status = "ERRO"
		}
		fmt.Printf("  %-5s %-10s %-40s %s\n", status, d.Class, d.Name, d.InstanceID)
	}
}

// closeHolderApps encerra os programas que seguram a porta da minitela.
func closeHolderApps() error {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return fmt.Errorf("listar processos: %w", err)
	}
	defer windows.CloseHandle(snap)

	self := uint32(os.Getpid())
	pe := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snap, &pe); err == nil; err = windows.Process32Next(snap, &pe) {
		name := windows.UTF16ToString(pe.ExeFile[:])
		if pe.ProcessID == self || !isHolderApp(name) {
			continue
		}
		fmt.Printf("Fechando %s (%d)\n", name, pe.ProcessID)
		if err := terminate(pe.ProcessID); errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			continue // saiu sozinho entre a listagem e o OpenProcess
		} else if err != nil {
			return fmt.Errorf("fechar %s (%d): %w", name, pe.ProcessID, err)
		}
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return fmt.Errorf("listar processos: %w", err)
	}
	return nil
}

func isHolderApp(exe string) bool {
	for _, a := range holderApps {
		if strings.EqualFold(exe, a) {
			return true
		}
	}
	return false
}

func terminate(pid uint32) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	if err := windows.TerminateProcess(h, 1); err != nil {
		return err
	}
	// espera o processo sair de fato (e liberar a porta) antes de reiniciar o dispositivo
	if ev, err := windows.WaitForSingleObject(h, 5000); err != nil {
		return err
	} else if ev != windows.WAIT_OBJECT_0 {
		return errors.New("o processo nao terminou em 5s")
	}
	return nil
}

// restartDevice usa o pnputil do Windows: desabilita e habilita num passo so, entao o
// dispositivo nao fica desabilitado se algo falhar no meio.
func restartDevice(instanceID string) error {
	pnputil := filepath.Join(os.Getenv("SystemRoot"), "System32", "pnputil.exe")
	out, err := exec.Command(pnputil, "/restart-device", instanceID).CombinedOutput()
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 3010 { // ERROR_SUCCESS_REBOOT_REQUIRED
		return errors.New("o Windows pediu reinicializacao para concluir o reinicio do dispositivo")
	}
	if err != nil {
		return fmt.Errorf("pnputil: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func resetCmd() int {
	isAdmin := windows.GetCurrentProcessToken().IsElevated()
	fmt.Println("Administrador:", isAdmin)

	devs, err := minitelaDevices()
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		return 1
	}
	if len(devs) == 0 {
		fmt.Printf("Dispositivo %s nao encontrado.\n", minitelaHWID)
		return 2
	}
	printDevices(devs)
	if !isAdmin {
		fmt.Println("Sem privilegio de administrador: nao e possivel reiniciar o dispositivo. Nenhum app foi fechado.")
		return 3
	}

	parent, ok := parentDevice(devs)
	if !ok {
		fmt.Println("Dispositivo composto nao encontrado (so as interfaces); nada foi reiniciado.")
		return 2
	}
	if err := closeHolderApps(); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		return 1
	}
	fmt.Println("Reiniciando:", parent.InstanceID)
	if err := restartDevice(parent.InstanceID); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		return 1
	}

	// pronto = composto e interfaces com status OK e a porta COM de volta no registro
	ready := false
	for deadline := time.Now().Add(15 * time.Second); !ready && time.Now().Before(deadline); {
		time.Sleep(500 * time.Millisecond)
		if devs, err = minitelaDevices(); err == nil {
			ready = allReady(devs, parent.InstanceID) && findPort() != ""
		}
	}
	printDevices(devs)
	if !ready {
		fmt.Println("Atencao: o dispositivo nao voltou com status OK em 15s.")
		return 4
	}
	fmt.Println("Concluido. Abra o app da minitela (ou segure a tecla dela por 2s) para testar.")
	return 0
}
