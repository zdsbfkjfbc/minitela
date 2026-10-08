# Referência do `mtc`

Detalhes de cada comando, códigos de saída, regras de XP e o funcionamento interno. Para instalar e começar a usar, veja o [README](../README.md).

## Comandos

| Comando | O que faz | Saída |
|---|---|---|
| `mtc page <1..7>` | Troca de página. | sempre 0 |
| `mtc note "<texto>"` | Escreve na página de Notas **sem** trocar de página. | sempre 0 |
| `mtc text "<texto>"` | Mostra o texto e troca para Notas. Só ASCII, até 100 caracteres; acentos são removidos. | sempre 0 |
| `mtc brightness <0..100>` | Brilho da tela (`0` apaga). | sempre 0 |
| `mtc xp start\|stop\|show\|note` | Motor de XP (lê o JSON do hook no stdin). | sempre 0 |
| `mtc startup [página]` | Espera a minitela voltar (boot ou sleep) e garante a página (padrão 1). | sempre 0 |
| `mtc run [-name rótulo] <cmd...>` | Roda o comando e mostra `OK build 12s` / `FALHOU build (1) 8s`. | a do comando (127 se não iniciar) |
| `mtc git [pasta]` | Branch, alterados e à frente/atrás na tela. | sempre 0 |
| `mtc handshake` | Testa a conexão. | 1 se falhar |
| `mtc get <registro>` | Lê um registro (2 = página atual). | 1 se falhar |
| `mtc flash <tema.acf> [página]` | Grava um tema (reinicia a minitela, ~25 s). | 1 se falhar |
| `mtc theme [opções]` | Compila o tema com os GIFs (não grava). | 1 se falhar |
| `mtc reset` | Reinicia o dispositivo USB (como Administrador). | 1, 2, 3 ou 4 (abaixo) |
| `mtc install` | Copia o `mtc` para `%LOCALAPPDATA%\Programs\mtc`. | 1 se falhar |

- Uso incorreto sai com **64**, nunca 2: nos hooks do Claude Code, o código 2 significa "erro bloqueante".
- Os comandos de hook nunca falham a sessão. Defina `MT_ECHO=1` para ver os erros e o texto enviado.
- `mtcw.exe` é o mesmo programa sem janela de console (para a tarefa agendada).
- A minitela aceita **um programa por vez**: feche o app oficial (`MiniTelaApp`) e o Minitela Go antes de usar `flash`, `handshake` ou `get`.

### `run`

Roda executáveis do PATH (`.exe`, `.cmd`, `.bat`). Para cmdlets do PowerShell, use `mtc run powershell -c "..."`. Só `-name`, e só como primeiro argumento, pertence ao `mtc`; as flags do comando passam intactas. O Ctrl+C chega ao comando, e o resultado ainda aparece na tela.

### `git`

Usa `git --no-optional-locks` (não disputa o `index.lock` com um git seu rodando ao mesmo tempo) e desiste após 10 s. Fora de um repositório, mostra `sem repositorio git`; outros erros aparecem como `git erro` (detalhes com `MT_ECHO=1`).

### `reset`

Fecha o `MiniTelaApp` e o Minitela Go e reinicia o dispositivo composto com `pnputil /restart-device`, sem alterar firmware nem imagens.

| Saída | Significado |
|---|---|
| 0 | Reiniciado; composto e interfaces voltaram com status OK. |
| 1 | Falha ao fechar um app ou ao reiniciar (inclui o Windows pedir reinicialização). |
| 2 | Dispositivo (ou o composto) não encontrado. |
| 3 | Sem privilégio de administrador; nada foi fechado. |
| 4 | O dispositivo não voltou com status OK em 15 s. |

### `flash`

Valida o tema **antes** de gravar e recusa, sem tocar na minitela, o que não for um `.acf` íntegro: o `.zip` do projeto, arquivo truncado, tamanho fora de `8 + N × 16384`, rodapé `0xA55A5AA5` ausente, cabeçalho incoerente ou checksum (XOR das palavras) inválido. As regras foram medidas nos 21 temas oficiais do app e em três temas próprios. Gravar um arquivo inválido trava a renderização e exige cortar a energia para recuperar.

**Recuperação:** `mtc flash backup-tema-fabrica\Texture.acf 3` volta ao tema de fábrica.

### `theme`

| Opção | Padrão |
|---|---|
| `-clawd-first` | desligado. Ligado, o Clawd vira a página 1 e o aparelho liga direto nele. |
| `-gifs <pasta>` | `pixelart\out` (precisa de `working.gif`, `done.gif`, `needs.gif`) |
| `-base <zip>` | `backup-tema-fabrica\file.zip` |
| `-workdir <pasta>` | `%LOCALAPPDATA%\MinitelaClawd` |

O compilador oficial (`AHMISimGenDemo_og.exe`) é achado pelo registro de pacotes do usuário, sem Administrador, e copiado para a pasta de trabalho (a pasta do app é somente leitura). A cópia é refeita quando o app é atualizado. O tema gerado passa pelas mesmas validações do `flash`.

Os caminhos padrão são relativos ao `bin\` do projeto: rode o `theme` pelo `bin\mtc.exe` do clone, ou passe `-base` e `-gifs` para a cópia instalada.

**O CRC do zip da Positivo:** o `file.zip` marca o bit 3 ("data descriptor") na maioria das entradas, mas não grava o descritor. O `archive/zip` do Go lê os 16 bytes seguintes como descritor e acusa `ErrChecksum`, embora o conteúdo e o CRC do diretório central estejam certos. O `theme` confere cada entrada contra o diretório central e copia as demais sem recomprimir, gerando um zip padrão.

### `install`

Copia `bin\mtc.exe`, `bin\mtcw.exe` e `xp\xp-config.json` para `%LOCALAPPDATA%\Programs\mtc`, mantendo o layout `bin\` + `xp\`. Lê todos os arquivos antes de trocar qualquer um, e consegue trocar um exe em uso por um hook (o antigo vira `.old` e some na próxima instalação). Rode de novo depois de recompilar ou de editar o `xp-config.json`.

## Páginas

Com o tema `-clawd-first`:

| Página | Conteúdo |
|---|---|
| 1 | Clawd trabalhando (ou a pele desbloqueada) |
| 2 | Notas (texto e progresso de XP) |
| 3 | Monitor do sistema |
| 4 | Clima |
| 5 | WhatsApp |
| 6 | Clawd terminou ("PRONTO!") |
| 7 | Clawd precisa de você |

No tema de fábrica, a página 1 é o WhatsApp e os GIFs ficam nas páginas 5, 6 e 7.

## XP e níveis

As regras ficam em `xp\xp-config.json`.

- **Por tarefa** (fim de uma resposta): 5, 15, 25, 40 ou 60 XP pela duração.
- **Bônus:** +50 na primeira tarefa do dia; +10% por dia seguido, até +50%.
- **Retorno decrescente por dia:** 100% até 400 XP, 50% até 800, 20% depois.
- **50 níveis:** LV10 = 800 XP, LV20 = 4.200, LV30 = 11.700, LV40 = 24.800, LV50 = 45.000.
- **Peles:** a cada 10 níveis uma pele nova é marcada como desbloqueada (`levelup.json`).

O progresso aparece na página de Notas (`LV5 [###-----] 205 XP`) sem trocar de página.

O estado fica em `%LOCALAPPDATA%\MinitelaClawd\xp-state.json`, gravado de forma atômica, com a versão anterior em `xp-state.json.bak`. Se o arquivo estiver corrompido, o `mtc` recupera do `.bak` e guarda o corrompido como `.corrompido-<data>`. Sem `.bak` válido, ele **não grava nada** e o arquivo fica intacto para recuperação manual.

## Protocolo

A minitela aparece no Windows como a porta serial `USB\VID_0324&PID_0324` (UART 115200, quadros `AH…MI` com CRC-16/ARC). As animações rodam **dentro da minitela**; o PC só diz qual página mostrar. A porta é achada pelo registro (instantâneo) em vez da detecção da biblioteca (~3,5 s). Um mutex global do Windows garante um processo por vez na porta.

A biblioteca de protocolo em `tools\mtc\minitela` é uma cópia sem alterações do [Minitela Go](https://github.com/EduardoSpek/minitela-positivo-golang) (MIT); a origem e o commit estão em `NOTICE.txt`.

## Organização do código

| Caminho | O que é |
|---|---|
| `tools\mtc\main.go` | Comandos de tela, porta e mutex. |
| `tools\mtc\flash.go` | `flash`, `get`, `handshake` e a validação do `.acf`. |
| `tools\mtc\theme.go` | `theme`: montagem do zip e chamada ao compilador. |
| `tools\mtc\run.go` | `run` e `git`. |
| `tools\mtc\reset.go` | `reset` (SetupAPI e `pnputil`). |
| `tools\mtc\install.go` | `install`. |
| `tools\mtc\xpcmd.go` | Liga o pacote de XP ao sistema (mutex, caminhos, terminal). |
| `tools\mtc\internal\xp` | Regras de XP sem dependência de Windows: `config.go` (carrega e valida), `rules.go` (a função pura `Apply`), `store.go` (estado), `format.go` (textos). |
| `pixelart\` | Gerador dos GIFs (192×192) em Go puro. |

Os resultados da última rodada de testes e análises estão em [ANALISE-E-TESTES.md](../ANALISE-E-TESTES.md).
