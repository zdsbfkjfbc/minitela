# Testes, análises e próximos passos

Registro da rodada de testes e análises de 08/10/2026 e da discussão sobre o PowerShell restante no projeto.

## Resultado dos testes e análises

| Verificação | Resultado |
|---|---|
| `gofmt`, `go vet` | Sem apontamentos. |
| `staticcheck` (versão em desenvolvimento, fixada em `v0.7.0-0.dev.0.20260824195211-6cb65e58a558`) | **Zero problemas**, inclusive na biblioteca copiada. A v0.7.0 estável não lê o Go 1.27. |
| `govulncheck` v1.8.0 | **Nenhuma vulnerabilidade alcançável.** Há uma em `golang.org/x/sys` v0.43.0 (GO-2026-5024, estouro de inteiro em `NewNTUnicodeString`), numa função que não chamamos, corrigida na v0.44.0. Recomendo atualizar. |
| Testes de unidade | Todos passando. A cobertura das regras de XP (`internal/xp`) subiu de 81% para **91%**, com testes novos para o texto da minitela e as faixas de duração. |
| Fuzzing (30 s por alvo) | **~2,1 milhões de execuções** em 4 alvos (`validateTheme`, `screenText`, `Apply`, `Load`), sem nenhuma falha. |
| Concorrência no XP | 30 tarefas simultâneas, 30 contadas, nenhuma perdida. |
| Concorrência na minitela | 12 comandos simultâneos em 434 ms, 0 erros, aparelho respondendo. |
| Uso real | 511 XP, nível 8, 7 sessões contando. A gravação nova já limpou o lixo antigo do arquivo de estado. |
| Latência | p50 de ~40 ms, p95 de ~50 ms (antes ~2.200 ms com PowerShell). |
| PowerShell (`PSScriptAnalyzer` 1.23) | Um problema real corrigido: acento num arquivo sem BOM no `mt.ps1`. O resto é convenção (`Write-Host` intencional, nome no plural). |

### Limitações

- **Não foi possível rodar o detector de corrida do Go** (`go test -race`), porque falta o `gcc`. O teste de concorrência real cobre a parte importante, mas não é equivalente.
- Houve dois erros nas próprias ferramentas de teste, não no código: uma expectativa de teste mal calculada (o progresso de 75 XP no nível 3 é 39%, não 75%) e uma medição no PowerShell em que o literal `0xA55A5AA5` virou um número negativo. Nos dois casos o código estava certo, e o teste foi corrigido.

## Migração concluída (08/10/2026, 2ª rodada)

O projeto agora está **só em Go**. Os cinco scripts PowerShell foram substituídos por comandos do `mtc` e apagados:

| Antes | Agora | Como foi verificado |
|---|---|---|
| `mt.ps1` | `mtc text/note/page/brightness` (já existiam) | — |
| `mt-run.ps1` | `mtc run [-name rotulo] <cmd...>` | Código de saída repassado (0, 3, 127), flags do comando intactas, testes de unidade. Corrige um erro do script: `[int](90/60)` arredondava para `2m30s`. |
| `mt-git.ps1` | `mtc git [pasta]` | Testes de tabela do resumo; erros do git deixam de virar "sem repositorio" (timeout de 10 s, `--no-optional-locks`). |
| `reset-minitela.ps1` | `mtc reset` (SetupAPI + `pnputil /restart-device`) | Rodado duas vezes no aparelho, como Administrador: reiniciou, voltou OK, handshake respondeu. |
| `pixelart\build-theme.ps1` | `mtc theme [-clawd-first]` | O `.acf` gerado é **idêntico** ao do script antigo, nos dois modos, exceto numa faixa de ~140 bytes (offsets 2245003–2265659) que o compilador muda **a cada execução** (duas execuções do próprio script diferem na mesma faixa). |

**O CRC do zip:** o `file.zip` da Positivo marca o bit 3 ("data descriptor") na maioria das entradas (o `data.json` e todos os PNGs), mas não grava o descritor, e o CRC do cabeçalho local é lixo. O `archive/zip` lê os 16 bytes seguintes (o próximo cabeçalho) como descritor e acusa `ErrChecksum`. O conteúdo e o CRC do diretório central estão certos. O `mtc theme` confere cada entrada contra o diretório central e copia as demais cruas (`OpenRaw`/`CreateRaw`), gerando um zip padrão.

**Revisão:** o código novo passou pelos agentes `go-reviewer` e `silent-failure-hunter`. Foram corrigidos: instalação parcial (agora em duas fases, com volta do arquivo antigo se a troca falhar), cópia incompleta do compilador sendo reaproveitada (agora via `Gen.tmp` + marcador de origem, recopiada quando o app atualizar), GIF inválido passando (agora `gif.DecodeConfig`), `data.json` revalidado com `json.Valid`, saída do compilador incluída no erro, `WaitDelay` contra pipe preso, `reset` dizendo "Concluido" cedo (agora exige composto e interfaces OK e espera o app fechar de fato), versão do app comparada numericamente, `LOCALAPPDATA` vazio não vira caminho relativo.

**Verificação final:** `go build`, `go vet`, `staticcheck` e `govulncheck` limpos; testes passando. Cobertura das funções puras novas entre 80% e 100%; o pacote `main` todo fica em 29% porque as partes que falam com o Windows (SetupAPI, registro, compilador, serial) foram verificadas executando no aparelho, não em teste de unidade. O `mtc.exe` passou de 3,6 para 4,2 MB (zip, flate e GIF); `xp show` roda em ~40 ms.

## Por que ainda tinha PowerShell? (histórico)

A resposta honesta é **histórico, mais dois motivos técnicos**. O projeto começou com o `reset-minitela.ps1`, e o Go entrou depois, quando o PowerShell se mostrou lento nos hooks (~80 MB e ~2 s por execução). Os scripts que sobraram:

| Script | Dá para passar para Go? |
|---|---|
| `mt.ps1` | **Já é redundante.** Só repassa para o `mtc.exe`. |
| `mt-run.ps1`, `mt-git.ps1` | **Sim, fácil.** Viram `mtc run` e `mtc git`. |
| `reset-minitela.ps1` | **Sim.** Ele usa os cmdlets de dispositivo do Windows (`Disable/Enable-PnpDevice`). Em Go dá para chamar o `pnputil /restart-device`, que vem no Windows. Continua exigindo admin. |
| `pixelart\build-theme.ps1` | **Sim, com um cuidado.** O `archive/zip` do Go rejeita o CRC do `data.json` desse projeto da Positivo (o autor do Minitela Go registrou isso, e por isso usou Python). O .NET do PowerShell tolera. Em Go seria preciso contornar, copiando as entradas cruas sem verificar o CRC, e conferir que o compilador oficial aceita o resultado. |

Passar tudo para Go deixaria o projeto em **uma linguagem só**, com os mesmos testes, `vet` e `staticcheck` valendo para tudo, sem a dependência do PowerShell 5.1 e suas armadilhas (BOM, literais hex virando negativos).

### Ordem sugerida para a migração

1. `mt.ps1`, `mt-run.ps1` e `mt-git.ps1` (mais simples).
2. `reset-minitela.ps1`.
3. `build-theme.ps1` (tem o detalhe do CRC do zip).

## Pendências

- ~~Atualizar `golang.org/x/sys` para v0.44.0 (GO-2026-5024).~~ Feito; `govulncheck` sem nenhuma vulnerabilidade.
- ~~`git init` com `.gitignore`.~~ Feito (`bin\`, `pixelart\out\`, `backup-tema-fabrica\` e `.claude\settings.local.json` fora). **Ainda sem o primeiro commit.**
- ~~Instalar o `mtc` num caminho estável.~~ Feito com `mtc install` em `%LOCALAPPDATA%\Programs\mtc`; os hooks globais e a tarefa `MinitelaClawdDefault` apontam para lá. Depois de recompilar ou editar o `xp-config.json`, rode `bin\mtc.exe install` de novo.
- `go test -race` continua sem rodar (falta o `gcc`).
- A biblioteca copiada (`tools\mtc\minitela`) está com quebras de linha CRLF, por isso o `gofmt -l` a lista; o código em si está formatado. Um `.gitattributes` com `*.go text eol=lf` resolve no primeiro commit.
