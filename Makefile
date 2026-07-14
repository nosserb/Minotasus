BINARY := kbdlight
PREFIX ?= /usr/local

.PHONY: build test run install install-udev clean

build:
	go build -o $(BINARY) ./cmd/kbdlight

test:
	go test ./...

run: build
	./$(BINARY)

# Installe le binaire dans $(PREFIX)/bin (nécessite les droits).
install: build
	install -Dm755 $(BINARY) $(PREFIX)/bin/$(BINARY)

# Installe la règle udev pour piloter le clavier sans sudo.
install-udev:
	sudo cp udev/99-asus-kbd-backlight.rules /etc/udev/rules.d/
	sudo udevadm control --reload-rules
	sudo udevadm trigger --subsystem-match=leds

clean:
	rm -f $(BINARY)
