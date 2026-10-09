PREFIX ?= MYCLI__
BINARY ?= exo
BINDIR ?= $(HOME)/.local/bin


# PREFIX=EXO make install BINARY=exo
.PHONY: install
install:
	mkdir -p "$(BINDIR)"
	go build -ldflags "-X mycli/app.EnvPrefix=$(PREFIX)" -o "$(BINDIR)/$(BINARY)" .
