## Downloads

| Arquivo | Para quê |
|---|---|
| **`minitela-{{VERSION}}-windows-amd64.zip`** | **Recomendado.** `mtc.exe`, `mtcw.exe`, a configuração de XP e os GIFs do Clawd (base e ninja) já gerados: dá para gravar o tema sem instalar o Go. |
| `mtc.exe` | O cliente de linha de comando, avulso. |
| `mtcw.exe` | O mesmo sem janela de console (para a tarefa agendada). |
| `SHA256SUMS.txt` | Somas para conferir os downloads. |

Os comandos de XP procuram a configuração em `..\xp\xp-config.json` ao lado do exe. Por isso, para o uso completo, prefira o zip, ou rode `mtc install` a partir dele.

## Começo rápido (sem Go)

1. Extraia o zip e abra um PowerShell na pasta extraída.
2. Teste a conexão: `bin\mtc.exe handshake` (feche o app oficial antes: a minitela aceita um programa por vez).
3. Copie os arquivos de fábrica do app oficial `PositivoMinitela` para `backup-tema-fabrica\`. O comando está no [README](https://github.com/{{REPO}}#2-copie-os-arquivos-de-f%C3%A1brica).
4. Compile e grave o tema: `bin\mtc.exe theme -clawd-first`, depois `bin\mtc.exe flash <caminho do Texture.acf> 1`. Para a pele ninja, acrescente `-gifs pixelart\out\ninja` ao `theme`.
5. Instale e configure os hooks: `bin\mtc.exe install`, e cole o JSON dos hooks do [README](https://github.com/{{REPO}}#4-instale-e-ligue-os-hooks) no `~/.claude/settings.json`.

## Observações

- Os executáveis **não são assinados**: o Windows SmartScreen pode avisar na primeira execução ("Mais informações" → "Executar assim mesmo"). Confira as somas em `SHA256SUMS.txt` com `Get-FileHash <arquivo>`.
- Somente Windows 10/11 x64, no Positivo Vision R15M.
- Projeto não oficial, sem vínculo com a Positivo Tecnologia nem com a Anthropic.
- Compilado pelo GitHub Actions a partir do commit {{SHA}}.
