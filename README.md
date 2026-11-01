# import-omarchy-key-bindings

Porta os atalhos do [Omarchy](https://omarchy.org/manual/hotkeys/) para um
**Ryoku com Hyprland**, de forma idempotente, auditável e reversível.

Foi escrito depois de fazer essa migração à mão: a parte cara não é digitar
binds, é descobrir os conflitos, mover o atalho deslocado do Ryoku para um
canto livre e acertar o clipboard universal (o `wtype` **não** funciona com o
`Super` fisicamente pressionado). Essa pesquisa está na tabela embutida e no
bloco Lua gerado.

```
$ import-omarchy-key-bindings plan
ACTION                    OMARCHY              RYOKU NOW            RESULT
Group windows (tabs)      SUPER + G            SUPER + T            moved
Copy                      SUPER + C            Centre               conflict resolved
...
```

## O que ele faz

1. Lê o catálogo vivo com `ryoku-hub keybinds` e o store de rebinds
   (`ryoku-hub desktop get`).
2. Resolve a tabela Omarchy → Ryoku contra o estado atual:
   - **same**: já coincide, não escreve nada;
   - **moved**: move um bind nativo do Ryoku para o chord do Omarchy;
   - **new**: cria o bind num chord livre;
   - **conflict resolved**: o chord do Omarchy estava ocupado, então o bind do
     Ryoku vai para um fallback (normalmente `Super+Alt+…` ou `Super+Ctrl+…`);
   - **not ported**: não existe ação equivalente, ou o app não está instalado.
3. Faz backup, aplica os rebinds pelo **comando dono**
   (`ryoku-hub desktop set-rebind`), reescreve só o bloco gerenciado de
   `user.lua`, roda `luac -p`, recarrega e confere `hyprctl configerrors` e os
   chords vivos.
4. Se qualquer passo falhar, restaura o backup e recarrega.

## Por que o clipboard é diferente

O Omarchy injeta o chord real com `send_key_state` e mods explícitos — não com
`wtype`. Com uma tecla física (`Super`) pressionada, um teclado virtual mistura
o `Super` na combinação e o app não recebe `Ctrl+C`. Em terminais, ele manda
`Ctrl+Shift+C/V`. O bloco gerado replica isso e detecta terminal por *tag*
(quando o compositor oferece) ou por classe da janela.

## Instalação

Requisitos: Ryoku com Hyprland (`ryoku wm status` → `Provider: hyprland`),
`ryoku-hub`, `hyprctl`, Go 1.23+. `luac` é opcional (usado na verificação).

```bash
git clone https://github.com/evandrojr/import-omarchy-key-bindings.git
cd import-omarchy-key-bindings
make build            # bin/import-omarchy-key-bindings
```

Se o seu `GOROOT` estiver quebrado (comum em setups com Go via tarball):
`GOROOT=/usr/lib/go make build`.

## Uso

```bash
import-omarchy-key-bindings plan      # não escreve nada; --json para máquina
import-omarchy-key-bindings apply     # pede confirmação; --yes para automação
import-omarchy-key-bindings verify    # luac + configerrors + chords vivos
import-omarchy-key-bindings restore   # volta o backup mais recente
import-omarchy-key-bindings mappings  # imprime a tabela curada
```

Flags úteis:

| Flag | Efeito |
|---|---|
| `--adopt` | assume binds escritos à mão em `user.lua` que estejam num chord mapeado (remove a linha e recria no bloco gerenciado) |
| `--omarchy-dir DIR` | lê `bindings/*.lua` de um checkout do Omarchy e segue os chords atuais (melhor esforço) |
| `--user-lua PATH` | outro `user.lua` |
| `--state-dir PATH` | outro diretório de backups |
| `--no-reload` | não recarrega o compositor (e pula a checagem de chords vivos) |
| `--json` | saída de `plan` em JSON |

`--adopt` existe para quem já tinha uma seção Omarchy à mão: sem ele, o tool se
recusa a encostar num bind manual. Ele só remove linhas `hl.bind(...)` de uma
linha, fora do bloco, cujo chord esteja na tabela; comentários e o resto do
arquivo ficam intactos.

## Onde ele escreve

| Arquivo | Como |
|---|---|
| `~/.config/hypr/rebinds.lua` | regenerado pelo `ryoku-hub desktop set-rebind` (nunca à mão) |
| `~/.config/ryoku/desktop.json` | store do `ryoku-hub`, chave `desktop.keybindRebinds` |
| `~/.config/hypr/user.lua` | só o bloco entre `>>> import-omarchy-key-bindings >>>` e `<<< ... <<<` |

Nada em `/usr/share/ryoku` nem em módulos distribuídos pelo Ryoku é tocado.
`Super+,` (Ryoku Settings) e `Super+Space` (launcher) são protegidos: o tool se
recusa a reivindicá-los.

## Fallbacks

Quando o chord do Omarchy já faz outra coisa no Ryoku, o bind nativo vai para:

| Bind do Ryoku | Fallback |
|---|---|
| Lock | `Super+Ctrl+L` (o próprio lock do Omarchy) |
| Clipboard | `Super+Ctrl+V` (o histórico do Omarchy) |
| Centre | `Super+Alt+C` |
| Overview | `Super+Ctrl+Tab` |
| Voice typing | `Super+Alt+grave` |
| Group | `Super+G` |
| Float | `Super+T` |
| Scratchpad | `Super+grave` |
| Wallpaper picker | `Super+Ctrl+W` (o background picker do Omarchy) |
| Window → monitor | `Super+Shift+Ctrl+seta` |
| Workspace → monitor | `Super+Shift+Alt+seta` |

Se um fallback curado estiver ocupado, o tool tenta acrescentar `Alt`, `Ctrl`,
`Shift` (e combinações) ao chord atual. Se não houver nada livre, ele **para**
em vez de sobrescrever.

## Backup e rollback

Cada `apply` cria
`~/.local/state/import-omarchy-key-bindings/backups/<timestamp>/` com
`hypr/user.lua`, `hypr/rebinds.lua` e `ryoku/desktop.json`, mais um
`manifest.json`. `restore` copia tudo de volta e recarrega. Falha em `luac`,
reload ou `configerrors` dispara o rollback automaticamente.

## Limitações

- Só Hyprland; em niri o tool se recusa a rodar.
- A tabela é curada (Omarchy docs + `bindings/*.lua`). O `--omarchy-dir` cobre
  mudanças de chord dos binds com rótulo, mas não inventa ações que o Ryoku não
  tem.
- Apps só entram se existirem no Ryoku; o tool não instala nada.
- `Super+Shift+Tab` depende de o Hyprland casar `Tab` com Shift no seu layout;
  confira uma vez.
- O clipboard usa `send_key_state`; layouts não-latinos podem precisar de ajuste
  de keysym.
- `F9` chama o dictation do Ryoku (`ryoku-shell voice`); o tool **não** baixa
  modelo nem liga o ditado — isso é feito em Ryoku Hub > Dictation.

## Config do herdr

`herdr/` guarda uma cópia da configuração da máquina de origem
(`~/.config/herdr/config.toml`, `~/.config/herdr-app/keys.json` e
`settings.json`), porque ela espelha o tmux do Omarchy. O tool **não** lê esses
arquivos; é referência/backup.

## Desenvolvimento

```bash
make test     # go test ./...
make vet
make fmt
```

Estrutura: `internal/chord` (normalização), `internal/keymap` (tabela
embutida), `internal/ryoku` (fronteira com o desktop), `internal/planner`
(resolução pura de conflitos), `internal/userlua` (bloco gerenciado),
`internal/render` (Lua gerado), `internal/backup`, `internal/omarchy`
(parser opcional).

## Licença

MIT.
