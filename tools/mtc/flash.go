package main

// Comandos de diagnostico e gravacao do tema: flash, get, handshake.
// Diferente dos comandos de hook, estes imprimem erros e saem com codigo != 0 quando falham.

import (
	"encoding/binary"
	"fmt"
	"os"
	"time"

	"mtc/minitela"
)

const (
	acfBlock  = 16384      // o compilador alinha o conteudo em blocos de 16 KB (+ 8 bytes de cabecalho)
	acfFooter = 0xA55A5AA5 // ultima palavra do arquivo e resultado do XOR de todas as palavras
)

// validateTheme recusa arquivos que nao sejam um tema .acf integro antes de gravar na flash:
// gravar algo invalido trava o render da minitela e exige cortar a energia para recuperar.
// Regras medidas em 24 temas validos (os 21 oficiais do app da Positivo + fabrica, Clawd base e
// ninja); o "magic" do cabecalho varia entre eles (0x8000 / 0x4000), por isso nao e verificado.
func validateTheme(data []byte) error {
	n := len(data)
	switch {
	case n >= 4 && string(data[:4]) == "PK\x03\x04":
		return fmt.Errorf("isto e um .zip (projeto), nao um tema compilado .acf; compile com: mtc theme")
	case n < 8+acfBlock || (n-8)%acfBlock != 0:
		return fmt.Errorf("tamanho %d bytes nao e de um tema .acf (esperado 8 + N x %d)", n, acfBlock)
	case n > minitela.MaxDownloadFileSize:
		return fmt.Errorf("tema de %d bytes passa do limite do aparelho (%d)", n, minitela.MaxDownloadFileSize)
	case binary.LittleEndian.Uint32(data[n-4:]) != acfFooter:
		return fmt.Errorf("rodape do tema ausente: arquivo truncado ou nao e um .acf")
	}
	// offset 2: numero de blocos de recurso de 64 KB; precisa existir e caber no arquivo.
	// Barra arquivos zerados com rodape, que passariam no XOR.
	if count := int(binary.LittleEndian.Uint16(data[2:])); count < 1 || acfBlock+count*65536 > n {
		return fmt.Errorf("cabecalho do tema invalido (%d blocos para %d bytes)", count, n)
	}
	var x uint32
	for i := 0; i < n; i += 4 {
		x ^= binary.LittleEndian.Uint32(data[i:])
	}
	if x != acfFooter {
		return fmt.Errorf("checksum invalido (XOR = %#08x): tema corrompido", x)
	}
	return nil
}

func handshake() error {
	return withClient(func(c *minitela.Client) error {
		max, err := c.Handshake()
		if err != nil {
			return err
		}
		fmt.Printf("handshake OK (maxPacketLength=%d)\n", max)
		return nil
	})
}

func getReg(reg uint16) error {
	return withClient(func(c *minitela.Client) error {
		m, err := c.GetNumTags([]uint16{reg})
		if err != nil {
			return err
		}
		fmt.Printf("registro %d = %d\n", reg, m[reg])
		return nil
	})
}

// flash grava um tema (.acf) em 0x08100000 e, apos o reinicio da minitela, mostra a pagina indicada.
// Restaurar a fabrica: mtc flash backup-tema-fabrica\Texture.acf 3
func flash(path string, page int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("erro lendo %s: %v", path, err)
	}
	if err := validateTheme(data); err != nil {
		return fmt.Errorf("%s recusado, nada foi gravado: %w", path, err)
	}
	err = withClient(func(c *minitela.Client) error {
		if _, err := c.Handshake(); err != nil {
			return fmt.Errorf("handshake falhou (feche o MiniTelaApp/Minitela Go: a minitela aceita um programa por vez): %w", err)
		}
		fmt.Printf("gravando %s (%d bytes) em %#x ...\n", path, len(data), minitela.FileTypeTexture)
		last := -10
		return c.UploadFile(data, minitela.FileTypeTexture, func(p int) {
			if p-last >= 10 || p == 100 {
				fmt.Printf("  %d%%\n", p)
				last = p
			}
		})
	})
	if err != nil {
		return fmt.Errorf("falha na gravacao: %w", err)
	}
	fmt.Println("gravacao concluida; aguardando a minitela reiniciar...")

	deadline := time.Now().Add(90 * time.Second)
	for {
		time.Sleep(3 * time.Second)
		err = withClient(func(c *minitela.Client) error {
			if _, err := c.Handshake(); err != nil {
				return err
			}
			if m, gerr := c.GetNumTags([]uint16{minitela.RegSystemPage}); gerr == nil {
				fmt.Printf("pagina ao ligar (registro 2): %d\n", m[minitela.RegSystemPage])
			}
			return c.SetPage(int32(page))
		})
		if err == nil {
			fmt.Printf("de volta. pagina %d exibida.\n", page)
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("a minitela nao voltou em 90s (ultimo erro: %v)", err)
		}
	}
}
