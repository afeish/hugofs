SHELL := /bin/bash
CURRENT_DIR=$(shell pwd)
BIN			= $(CURRENT_DIR)/bin
GO = go
MODULE   = $(shell env GO111MODULE=on $(GO) list -m)
PKGS     = $(or $(PKG),$(shell env GO111MODULE=on $(GO) list ./...))
TESTPKGS = $(shell env GO111MODULE=on $(GO) list -f \
			'{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' \
			$(PKGS))
TESTARGS = -gcflags="all=-N -l" --tags excludeTest -count 1 -v  -p $(shell nproc) -failfast
TARGET   = $(shell basename $(MODULE))
MAKEFLAGS+="-j $(shell nproc)"

TIMEOUT = 1800
# VERSION ?= $(shell if [ -z "`git status --porcelain`" ]; then git describe --exact-match --tags HEAD 2>/dev/null; fi)
VERSION ?= $(shell git describe --tags --always --dirty)
BUILDTIME ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GITCOMMIT ?= $(shell git rev-parse -q HEAD)
ifeq ($(CI_PIPELINE_ID),)
	BUILDNUMER := private
else
	BUILDNUMER := $(CI_PIPELINE_ID)
endif
LDFLAGS = -extldflags \
		  -static \
		  -X "main.Version=$(VERSION)" \
		  -X "main.BuildTime=$(BUILDTIME)" \
		  -X "main.GitCommit=$(GITCOMMIT)" \
		  -X "main.BuildNumber=$(BUILDNUMER)"
GOPROXY = https://goproxy.cn,direct
V ?= 0
Q = $(if $(filter 1,$V),,@)
M = $(shell printf "\033[34;1m▶\033[0m")

define whatisit
$(info $(1) origin is ($(origin $(1))) and value is ($($(1))))
endef


ifeq ($(V),1)
$(call whatisit,TESTPKGS)
$(call whatisit,TARGET)
$(call whatisit,MODULE)
$(call whatisit,VERSION)
endif

hugo: clean
	$Q echo "build hugo"
	$Q git submodule update --init
	$Q GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -gcflags=all="-N -l"  -o ./bin/hugofs .
	$Q echo "build finished"

install: hugo
	$Q echo "install hugofs"
	$Q sudo install -m 0755 ./bin/hugofs /bin/

# Tools
$(BIN):
	$Q mkdir -p $@

$(BIN)/%: | $(BIN) ; $(info $(M) building $(PACKAGE))
	tmp=$$(mktemp -d); \
	GOPATH=$$tmp env GO111MODULE=on GOPROXY=https://goproxy.cn,direct GOBIN=$(BIN) go install $(PACKAGE) \
		|| ret=$$?; \
	chmod 700 -R $$tmp; rm -rf $$tmp ; exit $$ret

GODLV = $(BIN)/dlv
$(BIN)/dlv: PACKAGE=github.com/go-delve/delve/cmd/dlv@latest


compile: hugo

cloc:
	$Q echo "Statistic Code"
	$Q cloc --exclude-ext=html,css,js,json,svg,XML,xml --exclude-dir docs --exclude-dir pb .

book:
	mdbook serve docs

init_chglog:
	$Q echo "Install & Init git-chglog"
	$Q go get -u github.com/git-chglog/git-chglog/cmd/git-chglog
	$Q git-chglog --init
	$Q echo "Install Finished"

chglog:
	$Q echo "Update CHANGELOG: $(BUILD_DATE)"
	$Q echo "Git tag: $(GIT_TAG)"
	$Q echo "Current Directory $(CURRENT_DIR)"
	$Q git-chglog -o CHANGELOG.md
	$Q echo "UPDATE Finished"

.PHONY: proto
proto:
	$Q echo "start compile proto"
	$Q buf generate --template proto/buf.yml -o . proto
	$Q echo "compile finished"

godoc:
	$Q echo "Build GODOC"
	$Q echo "http://localhost:6060/pkg/github.com/afeish/hugo"
	$Q godoc -http=:6060 -goroot $(CURRENT_DIR)

check:
	$Q echo "Static Check"
	$Q echo "Ensure you have installed golangci-lint already"
	$Q GO111MODULE=on CGO_ENABLED=0 golangci-lint run -v --config .golangci.yml

tikv:
	$Q echo "Setup TiKV cluster"
	$Q tiup playground --mode tikv-slim

mount:
	$Q echo "Mount hugo"
	$Q ./bin/hugofs mount

umount:
	$Q umount -l /tmp/hugofs
	$Q file /tmp/hugo
	$Q ls /tmp/hugo

docker:
	docker build -t neocxf/hugofs-meta -f build/meta.dockerfile .
	docker build -t neocxf/hugofs-client -f build/client.dockerfile .
	docker build -t neocxf/hugofs-storage -f build/storage.dockerfile .

docker.push: docker
	docker push neocxf/hugofs-meta
	docker push neocxf/hugofs-client
	docker push neocxf/hugofs-storage

debug: hugo

define docker_up
    ./misc/fix-compose.sh
	docker-compose -f build/docker-compose-dev.yaml stop -t 3
	docker-compose -f build/docker-compose-dev.yaml up --build --force-recreate --remove-orphans -d
	docker exec -it g_s1 hugofs s v a -p /mnt/1-1 2>&1 >/dev/null
	docker exec -it g_s2 hugofs s v a -p /mnt/2-1 2>&1 >/dev/null
	docker exec -it g_s3 hugofs s v a -p /mnt/3-1 2>&1 >/dev/null
	docker exec -it g_c1 bash -c "cd /root/pjdfstest/ &&  autoreconf -ifs && ./configure && make pjdfstest && ln -s /root/pjdfstest/pjdfstest /bin/pjdfstest"
endef

dev: hugo $(GODLV)
	$(call docker_up)

dev-logs:
	docker-compose -f build/docker-compose-dev.yaml logs -f

dev-clean:
	docker-compose -f build/docker-compose-dev.yaml down -v

clean:
	$Q rm -rf /tmp/gile_mock_db/ /tmp/hugo_cache bin/hugofs dist/; \
	find . -type f -name "*.test" -delete; \
	find . -type f -name "__debug_bin*" -delete


# TESTPKGS := github.com/afeish/hugo/pkg/hugofs/state/iobuffer
TEST_TARGETS := test-default test-bench test-short test-verbose test-race

test-bench:   TESTARGS += -benchtime=1000x -bench=. ## Run benchmarks
test-short:   TESTARGS += -short        ## Run only short tests
test-verbose: TESTARGS += -v            ## Run tests in verbose mode with coverage reporting
test-race:    TESTARGS += -race  ## Run tests with race detector
$(TEST_TARGETS): NAME=$(MAKECMDGOALS:test-%=%)
$(TEST_TARGETS): test
test:; $(info $(M) running $(NAME:%=% )tests) @ ## Run tests
	$Q $(GO) test -timeout $(TIMEOUT)s $(TESTARGS) $(TESTPKGS)


snapshot: clean
	export GORELEASER_CURRENT_TAG=v0.0.0
	goreleaser release --snapshot --clean
	curl -v -k -u "admin:X7Eh@C20pw" -H "Content-Type: multipart/form-data" --data-binary "@dist/hugo-v0.0.0-linux-amd64.deb" "http://nexus.neochen.store/repository/ellis.chen-apt/"
	curl -v -k -u "admin:X7Eh@C20pw" --upload-file "dist/hugo-v0.0.0-linux-amd64.rpm" "http://nexus.neochen.store/repository/ellis.chen-yum/hugo/"

# password is hugo
release-s3: clean
	export GORELEASER_CURRENT_TAG=v0.0.0
	goreleaser release --snapshot --clean
	rclone --config misc/rclone.conf copy dist/hugo-v0.0.0-linux-amd64.deb ellis.chen:terraform-hugo/bin

release: clean
	goreleaser release

pjdfstest.build:
	docker build -t neochen.store/tools/pjdfstest-build -f build/pjdfstest.dockerfile .
	docker push neochen.store/tools/pjdfstest-build

pjdfstest.run: hugo
	pushd deploy/pjdfstest && autoreconf -ifs && ./configure && make pjdfstest && popd
	hugo_root=$(shell pwd); tmp=$$(mktemp -d); hugo_LOG_LEVEL=panic $$hugo_root/bin/hugofs test self-contained --mp $$tmp --daemon; sleep 5; \
		pushd $$tmp && prove -rv $$hugo_root/deploy/pjdfstest/tests || ret=$$?; \
		pkill -INT hugo; exit $$ret

TESTDEBUGFLAGS := -gcflags "all=-N -l" --tags excludeTest -count 1 -v  -p $(shell nproc)

DEBUGPKG := github.com/afeish/hugo/tests
# ./iobuffer.test -test.v -test.run "^TestRWBuffer$" -testify.m "^(TestDyncmicTask)$"
testbin: clean
	@PKG_DIR='$(subst $(MODULE)/,,$(DEBUGPKG))'; \
	echo $$PKG_DIR; \
	PKG_DIR=$(shell go list -f '{{.Dir}}' $(DEBUGPKG)); \
	echo $$PKG_DIR; \
	TEST_BIN=$(shell go list -f '{{.Name}}' $(DEBUGPKG)); \
	echo $$TEST_BIN; \
	go test $(TESTDEBUGFLAGS) -c -o $$PKG_DIR/$$TEST_BIN.test $(DEBUGPKG); \
	# cd $$PKG_DIR; $$PKG_DIR/$$TEST_BIN.test -test.v -test.run "^TestRWBuffer$$" -testify.m "^(TestDyncmicTask)$$" \
	cd $$PKG_DIR; $$PKG_DIR/$$TEST_BIN.test -test.v -test.failfast -test.memprofile memprofile.out -test.cpuprofile profile.out
