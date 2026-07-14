# Minotasus — kbdlight

Petit programme en **Go** pour allumer et régler le **rétroéclairage du clavier**
d'un portable **ASUS Vivobook** sous Linux. Tu le lances, tu appuies sur une
touche, la lumière du clavier s'allume.

## Comment ça marche

Sur les portables ASUS, le module noyau `asus_nb_wmi` expose le rétroéclairage
dans le sysfs :

```
/sys/class/leds/asus::kbd_backlight/brightness      # niveau courant (0 = éteint)
/sys/class/leds/asus::kbd_backlight/max_brightness  # niveau max (souvent 3)
```

Le programme lit et écrit simplement ce fichier. Aucune dépendance externe :
uniquement la bibliothèque standard Go (le mode « touche par touche » du
terminal est géré via `stty`).

## Prérequis

- Un portable ASUS avec `/sys/class/leds/asus::kbd_backlight` présent
  (vérifie avec `lsmod | grep asus_nb_wmi`).
- Go 1.24+ pour compiler.

## Compilation

```bash
make build      # produit ./kbdlight
# ou
go build -o kbdlight ./cmd/kbdlight
```

## Utilisation

```bash
./kbdlight
```

Touches dans le programme :

| Touche      | Effet                          |
|-------------|--------------------------------|
| `espace`    | allume / éteint (bascule)      |
| `+` / `-`   | augmente / diminue le niveau   |
| `0`–`3`     | règle directement le niveau    |
| `q`         | quitte                         |

## Droits d'écriture

Le fichier `brightness` appartient à `root`. Deux options :

**1. Rapide** — lancer avec sudo :

```bash
sudo ./kbdlight
```

**2. Propre** — installer la règle udev une fois, puis utiliser sans sudo :

```bash
make install-udev      # copie la règle et recharge udev
```

Cette règle donne au groupe `plugdev` le droit d'écrire le niveau. Vérifie que
tu es dans ce groupe (`id | grep plugdev`).

## Développement

```bash
make test      # tests unitaires du package backlight
go vet ./...
```

## Structure

```
cmd/kbdlight/         programme interactif (terminal)
internal/backlight/   package de lecture/écriture du LED sysfs
udev/                 règle udev pour l'usage sans sudo
```
