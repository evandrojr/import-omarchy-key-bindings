# Voxtype: ditado no F9, modelo pt e GPU

Notas da máquina de referência (Ryoku/Hyprland, outubro/2026). O F9 chama
`ryoku-shell voice`, que fala com o daemon do voxtype. Sem um modelo
multilíngue o Whisper devolve transcrição vazia para português; sem GPU ele
fica lento.

## Modelo e idioma

- Pacote: `voxtype-bin 1.1.1`.
- Modelo: `base` multilíngue (`~/.local/share/voxtype/models/ggml-base.bin`,
  141 MB), preset Ryoku `whisper-base`.
- `whisper.language = "pt"` em `~/.config/voxtype/config.toml`.
- O `base.en` (English-only) produz `degenerate transcript ""` com fala em
  português — foi o primeiro sintoma.

## GPU

Hardware: Intel HD 520 (iGPU) + NVIDIA GeForce 920MX, drivers i915 + nvidia
580xx. A variante Vulkan já vem no pacote, mas o daemon sobe em AVX2 por
padrão.

Tempos para a mesma frase pt-BR (~4 s de áudio), incluindo carga do modelo:

| Variante | Tempo |
|---|---|
| AVX2 (CPU) | 13,54 s |
| Vulkan "auto" (escolheu NVIDIA) | 9,80 s |
| **Vulkan na Intel** | **2,59 s** |
| Vulkan na NVIDIA 920MX | 11,93 s |

O `auto` pega a discreta NVIDIA e perde para a Intel; por isso a seleção é
explícita.

### Drop-in (sem sudo)

`~/.config/systemd/user/voxtype.service.d/gpu.conf`:

```ini
[Service]
Environment=VOXTYPE_VULKAN_DEVICE=intel
ExecStart=
ExecStart=/usr/lib/voxtype/voxtype-vulkan daemon
```

Aplicar:

```bash
systemctl --user daemon-reload
systemctl --user restart voxtype
```

Conferir:

```bash
ps -o cmd= -p "$(cat /run/user/1000/voxtype/pid)"
tr '\0' '\n' < /proc/"$(cat /run/user/1000/voxtype/pid)"/environ | grep VOXTYPE
voxtype info accel        # State: gpu / Backend: vulkan
journalctl --user -u voxtype -n 20 --no-pager | grep Vulkan
```

Reverter: apague o `gpu.conf`, `daemon-reload`, `restart voxtype`.

O symlink `/usr/bin/voxtype` continua em `voxtype-avx2` (é root). O daemon é
Vulkan pelo drop-in. Se rodar `sudo voxtype setup gpu --enable`, mantenha o
`VOXTYPE_VULKAN_DEVICE=intel`, senão o auto pode escolher a NVIDIA.

### Ajustes opcionais

```bash
voxtype config set whisper.on_demand_loading false  # modelo residente, menos latência
voxtype config set whisper.eager_processing true    # transcreve enquanto fala
```
