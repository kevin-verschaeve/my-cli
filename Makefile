PREFIX ?= MYCLI__
BINARY ?= exo
BINDIR ?= $(HOME)/.local/bin


# PREFIX=EXO make install BINARY=exo
# go version -m "$(command -v exo)" ## voir les options de compilation
.PHONY: install
install:
	mkdir -p "$(BINDIR)"
	go build -ldflags "-X mycli/app.EnvPrefix=$(PREFIX)" -o "$(BINDIR)/$(BINARY)" .
