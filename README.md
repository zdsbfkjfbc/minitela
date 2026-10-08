# minitela

Controle da **minitela** (LCD 1,54" 240×240 abaixo do teclado) do notebook Positivo Vision R15M: animações do Clawd em pixel art que reagem ao Claude Code, XP e níveis por tarefa concluída, e utilitários.

A minitela aparece no Windows como a porta serial `USB\VID_0324&PID_0324` (UART 115200, quadros `AH…MI`). O protocolo vem do projeto [Minitela Go](https://github.com/EduardoSpek/minitela-positivo-golang) (MIT), cuja biblioteca está copiada em `tools\mtc\minitela`. As animações rodam **dentro da minitela**; o PC só diz qual página mostrar.

## Estrutura

| Caminho | O que é |
|---|---|
| `bin\mtc.exe`, `bin\mtcw.exe` | Cliente único (Go): hooks, XP, acordar, diagnóstico, compilação e gravação de tema, utilitários. `mtcw` é o mesmo sem janela de console. Gerados por `go build` (fora do git). |
| `%LOCALAPPDATA%\Programs\mtc\` | Cópia instalada (`mtc install`) usada pelos hooks e pela tarefa agendada. |
| `tools\mtc\` | Código-fonte do `mtc` e cópia da biblioteca de protocolo. |
| `xp\xp-config.json` | Curva de níveis, XP por tarefa, bônus e faixas diárias. |
| `pixelart\` | Gerador dos GIFs (Go). |
| `backup-tema-fabrica\` | Tema de fábrica e temas de recuperação (proprietários da Positivo, fora do git). **Não apague.** |
| `.claude\settings.local.json` | Hooks deste projeto (animações). |

## Páginas da minitela

`1` Clawd/ninja trabalhando · `2` Notas · `3` Monitor · `4` Clima · `5` WhatsApp · `6` terminou ("PRONTO!") · `7` precisa de você. O aparelho liga na página 1.

## `mtc.exe`

```powershell
bin\mtc.exe page 6                # troca de página
bin\mtc.exe note "texto"          # escreve em Notas SEM trocar de página
bin\mtc.exe text "texto"          # mostra texto (troca para Notas); só ASCII, até 100 caracteres, acentos são removidos
bin\mtc.exe brightness 60         # 0 apaga a tela
bin\mtc.exe xp show               # nível, XP, % para o próximo, streak
bin\mtc.exe handshake             # testa a conexão
bin\mtc.exe get 2                 # lê um registro (2 = página atual)
bin\mtc.exe startup 1             # espera a minitela voltar (boot/acordar) e garante a página
bin\mtc.exe flash <tema.acf> 1    # grava um tema (reinicia a minitela, ~25 s)
bin\mtc.exe theme -clawd-first    # compila o tema com os GIFs do Clawd (NÃO grava); ver "Animações e peles"
bin\mtc.exe run go build ./...    # roda o comando e mostra "OK build 12s" / "FALHOU build (1) 8s" na tela
bin\mtc.exe run -name testes npm test
bin\mtc.exe git [pasta]           # branch, alterados e a frente/atrás na tela
bin\mtc.exe reset                 # reinicia o dispositivo USB (como Administrador)
bin\mtc.exe install               # copia mtc.exe, mtcw.exe e xp-config.json para %LOCALAPPDATA%\Programs\mtc
```

Cerca de 2,5 MB e 0,07 s por execução. Comandos de hook (e o `git`) sempre saem com código 0; `MT_ECHO=1` mostra erros e o texto enviado. Uso incorreto sai com 64 (nunca 2, que nos hooks do Claude Code significa "erro bloqueante"); `handshake`, `get`, `flash`, `theme` e `install` saem com 1 se falharem.

O `run` devolve o código de saída do comando (127 se ele não iniciar). Ele roda executáveis do PATH (`.exe`, `.cmd`, `.bat`); para cmdlets do PowerShell use `mtc run powershell -c "..."`. Só `-name`, e como primeiro argumento, é do `mtc`; as flags do comando passam intactas.

O `reset` fecha o `MiniTelaApp` e o Minitela Go (a porta aceita um programa por vez) e reinicia o dispositivo composto com o `pnputil /restart-device`, sem alterar firmware nem imagens. Saídas: 1 falha ao fechar apps ou reiniciar (inclui o Windows pedir reinicialização), 2 dispositivo (ou o composto) não encontrado, 3 sem privilégio de administrador (nada é fechado), 4 o composto e as interfaces não voltaram com status OK em 15 s.

O `flash` **valida o tema antes de gravar** e recusa, sem tocar na minitela, o que não for um `.acf` íntegro: o `.zip` do projeto, arquivo truncado, tamanho fora de `8 + N × 16384`, rodapé `0xA55A5AA5` ausente, cabeçalho incoerente ou checksum (XOR das palavras) inválido. As regras foram medidas nos 21 temas oficiais do app e nos nossos três. A minitela aceita **um programa por vez**: feche o `MiniTelaApp` e o Minitela Go antes de usar.

Recompilar (requer Go): `cd tools\mtc; go build -trimpath -ldflags "-s -w" -o ..\..\bin\mtc.exe .` (para o `mtcw.exe`, acrescente `-H=windowsgui`). Testes: `cd tools\mtc; go test ./...`. **Depois de recompilar ou editar o `xp\xp-config.json`, rode `bin\mtc.exe install`**: os hooks globais e a tarefa agendada usam a cópia instalada, que não se atualiza sozinha. O `install` troca até um exe em uso (renomeia o antigo para `.old`).

Organização do código: `internal\xp` tem as regras de XP sem dependência de Windows (`config.go` carrega e valida a configuração, `rules.go` é a função pura `Apply`, `store.go` grava e lê o estado, `format.go` monta os textos), com testes; `xpcmd.go` liga isso ao sistema (mutex, caminhos, terminal).

## Hooks do Claude Code

- **Neste projeto** (`.claude\settings.local.json`): `UserPromptSubmit` → página 1, `Stop` → página 6, `Notification` → página 7 (todos `async`).
- **Globais** (`~/.claude/settings.json`): `UserPromptSubmit` → `mtc xp start` e `Stop` → `mtc xp stop`, para contar XP de **todas** as sessões. Apontam para a cópia instalada (`%LOCALAPPDATA%\Programs\mtc\bin\mtc.exe`), então continuam funcionando se a pasta do projeto for movida.

## XP e níveis

Cada tarefa concluída (fim de uma resposta) rende XP: 5/15/25/40/60 pela duração, +50 na primeira do dia, +10% por dia seguido (até +50%), com retorno decrescente por dia (100% até 400 XP, 50% até 800, 20% depois). 50 níveis: LV10 = 800 XP, LV20 = 4.200, LV30 = 11.700, LV40 = 24.800, LV50 = 45.000. O progresso aparece na página de Notas (`LV5 [###-----] 205 XP`) sem trocar de página. A cada 10 níveis uma pele nova é marcada como desbloqueada (`levelup.json`). Estado em `%LOCALAPPDATA%\MinitelaClawd\xp-state.json`, gravado de forma atômica com a versão anterior em `xp-state.json.bak`. Se o arquivo estiver corrompido, o `mtc` recupera do `.bak` (e guarda o corrompido como `.corrompido-<data>`); sem `.bak` válido, ele **não grava nada** e o arquivo fica intacto para recuperação manual (`MT_ECHO=1` mostra o motivo).

## Animações e peles

```powershell
cd pixelart
go run .                    # Clawd base  -> out\working.gif, done.gif, needs.gif
go run . -skin ninja        # pele ninja (nível 10) -> out\ninja\*.gif   (-sheets gera folhas de prévia em PNG)
..\bin\mtc.exe theme -clawd-first -gifs out\ninja   # compila o tema, NÃO grava (~15 s)
..\bin\mtc.exe flash <Texture.acf mostrado no final> 1   # grava
```

O `theme` usa o compilador do app oficial `PositivoMinitela` (Microsoft Store, achado pelo registro do usuário, sem Administrador) e o `backup-tema-fabrica\file.zip`; `-clawd-first` coloca o Clawd como primeira página, então o aparelho liga direto nele. Outras opções: `-gifs <pasta>` (padrão `pixelart\out`), `-base <zip>`, `-workdir <pasta>` (padrão `%LOCALAPPDATA%\MinitelaClawd`). O tema gerado passa pelas mesmas validações do `flash` antes de ser entregue. Rode o `theme` pelo `bin\mtc.exe` do projeto: a cópia instalada não sabe onde fica o backup (a não ser com `-base` e `-gifs`).

O `file.zip` da Positivo marca "data descriptor" em várias entradas sem gravá-lo, e o `archive/zip` do Go acusa erro de CRC nelas, embora o conteúdo esteja certo. O `theme` confere cada entrada contra o CRC do diretório central e copia as demais sem recomprimir, gerando um zip padrão. O `.acf` resultante é idêntico ao que o antigo `build-theme.ps1` gerava, salvo uma faixa de metadados que o próprio compilador muda a cada execução.

O tema de fábrica e o `file.zip` são da Positivo e não estão no repositório: copie-os do app oficial `PositivoMinitela` (`MiniTelaApp\assets\minipanel\resources\IDE_utils_pt\Zip\file.zip` e `...\IDE_utils_pt\ACF\Texture.acf`, dentro da pasta de instalação do app) para `backup-tema-fabrica\` antes de usar o `theme` ou de restaurar.
**Recuperação:** `bin\mtc.exe flash backup-tema-fabrica\Texture.acf 3` (fábrica) ou `...\Texture_clawd-base.acf 1` (Clawd base).

## Ao ligar/acordar

A tarefa agendada `MinitelaClawdDefault` (só do seu usuário) roda `%LOCALAPPDATA%\Programs\mtc\bin\mtcw.exe startup 1` ao entrar no Windows e ao acordar do sleep (evento `Power-Troubleshooter 1`), como rede de segurança caso o aparelho abra em outra página.

```powershell
Start-ScheduledTask MinitelaClawdDefault
Unregister-ScheduledTask MinitelaClawdDefault -Confirm:$false   # remover
```

## Licença

MIT (veja `LICENSE`). A biblioteca em `tools\mtc\minitela` é do [Minitela Go](https://github.com/EduardoSpek/minitela-positivo-golang), também MIT (veja `tools\mtc\minitela\NOTICE.txt`). As skills e agentes em `.claude\` vêm do ECC (MIT, `.claude\LICENSE-ECC.txt`).
