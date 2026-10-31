# Onde a tabela veio

Fontes usadas para montar `internal/keymap/keymap.json`:

- Omarchy — Hotkeys: <https://omarchy.org/manual/hotkeys/>
- Omarchy — Unified Clipboard & History:
  <https://omarchy.org/manual/unified-clipboard-history/>
- Omarchy — `default/hypr/bindings/clipboard.lua`:
  <https://github.com/omacom/omarchy/blob/quattro/default/hypr/bindings/clipboard.lua>
- Omarchy — `default/hypr/apps/terminals.lua`:
  <https://github.com/omacom/omarchy/blob/quattro/default/hypr/apps/terminals.lua>
- Ryoku — Keybindings: <https://docs.ryoku.dev/docs/keybindings>
- Ryoku — a lista que realmente vale, na máquina:
  `~/.config/hypr/modules/binds.lua`, `ryoku-hub keybinds` e `hyprctl binds`.

## Decisões que não são óbvias

**Clipboard.** O Omarchy não usa `wtype`. Ele chama
`hl.dsp.send_key_state({mods, key, state})` com `down` e, 50 ms depois, `up`.
O comentário no `clipboard.lua` explica: uma tecla virtual faz o `Super`
fisicamente pressionado entrar na combinação injetada. Em terminal, o chord vira
`Ctrl+Shift+C/V`. O bloco gerado por este projeto replica isso.

**Terminais.** O Omarchy marca janelas com a tag `terminal`
(`apps/terminals.lua`). O Ryoku não faz isso, então o fallback é a classe da
janela (`kitty`, `Alacritty`, `com.github.wez.wezterm`, …), com a tag ainda
preferida quando existir.

**Protegidos.** `Super+,` é Ryoku Settings e `Super+Space` é o launcher; os dois
são preservados mesmo quando o Omarchy usa o chord para outra coisa.

**Não portados.** Ações do menu do Omarchy (`Super+Space`), painéis
(`Super+Ctrl+A/B/N/D/P`), ciclo de layouts e apps ausentes ficam de fora e
aparecem no relatório como `not ported`, com o motivo.

## Documentação pode envelhecer

A página de hotkeys do Omarchy e a instalação variam. Por isso existe
`--omarchy-dir`: ele lê `bindings/*.lua` de um checkout e, para os binds que
têm `omarchy_label` na tabela (copy/cut/paste), segue o chord que estiver lá.
O resto da tabela continua curado — de propósito, porque mapear *ação* exige
saber o que o Ryoku tem.
