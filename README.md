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

### Plus de niveaux que le matériel (PWM logiciel)

Le clavier n'a que 4 crans (0, 1, 2, 3). Pour obtenir des niveaux
**intermédiaires**, le programme **clignote très vite entre deux crans** : par
exemple 50 % du temps à 2 et 50 % à 1 donne une luminosité perçue de « 1,5 ».
En jouant sur ce rapport, on obtient un continuum. Chaque cran matériel est
découpé en 4, soit **13 niveaux perçus** au lieu de 4.

Le clignotement doit être assez rapide pour être invisible. Deux leviers :

- **`-period`** (défaut 15 ms) : durée d'un cycle. Plus court = fréquence plus
  haute = moins de scintillement.
- **`-pulses`** (défaut `0` = auto) : découpe chaque cycle en plusieurs
  impulsions courtes au lieu d'un seul flash, ce qui monte encore la fréquence
  perçue — c'est surtout ça qui règle le scintillement des niveaux sombres.

En mode auto, le programme **mesure la latence d'écriture réelle** de ton
clavier au démarrage et en déduit combien d'impulsions il peut faire sans
saturer le contrôleur (l'affichage indique la valeur retenue). Tu peux forcer :

```bash
./kbdlight -pulses 6            # plus d'impulsions si ça scintille encore
./kbdlight -period 10ms         # cycle plus court
./kbdlight -pulses 1            # revenir à un seul flash par cycle
```

Selon le contrôleur du clavier, un léger scintillement peut rester sur certains
niveaux : c'est la limite d'un PWM piloté depuis l'espace utilisateur (chaque
écriture passe par l'ACPI/WMI, ce qui borne la fréquence atteignable).

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

| Touche      | Effet                                   |
|-------------|-----------------------------------------|
| `espace`    | allume / éteint (bascule)               |
| `+` / `-`   | monte / descend d'un cran fin (PWM)     |
| `0`–`3`     | règle directement un cran matériel plein|
| `q`         | quitte                                  |

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
internal/effect/      PWM logiciel (niveaux intermédiaires par clignotement)
udev/                 règle udev pour l'usage sans sudo
```
