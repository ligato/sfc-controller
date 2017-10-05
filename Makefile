VERSION=$(shell git rev-parse HEAD)
DATE=$(shell date +'%Y-%m-%dT%H:%M%:z')
LDFLAGS=-ldflags '-X wwwin-gitlab-sjc.cisco.com/ctao/sfc-controller/vendor/github.com/ligato/cn-infra/core.BuildVersion=$(VERSION) -X wwwin-gitlab-sjc.cisco.com/ctao/sfc-controller/vendor/github.com/ligato/cn-infra/core.BuildDate=$(DATE)'

PLUGIN_SOURCES="sfc_controller.go"
PLUGIN_BIN="sfc_controller.so"
ETCD_CONFIG_FILE="etcd/etcd.conf"
KAFKA_CONFIG_FILE="kafka/kafka.conf"
SFC_CONFIG_FILE="sfc.conf"

# generate go structures from proto files & binapi json files
define generate_sources
        $(if $(shell command -v protoc --gogo_out=. 2> /dev/null),$(info gogo/protobuf is installed),$(error gogo/protobuf missing, please install it with go get github.com/gogo/protobuf))
        @echo "# generating sources"
        @go generate -v
        @cd plugins/vnfdriver && go generate -v
        @echo "# done"
endef

# install dependencies according to glide.yaml & glide.lock (in case vendor dir was deleted)
define install_dependencies
	$(if $(shell command -v glide install 2> /dev/null),$(info glide dependency manager is ready),$(error glide dependency manager missing, info about installation can be found here https://github.com/Masterminds/glide))
	@echo "# installing dependencies, please wait ..."
	@glide install --strip-vendor
endef

# clean update dependencies according to glide.yaml (re-downloads all of them)
define update_dependencies
	$(if $(shell command -v glide install 2> /dev/null),$(info glide dependency manager is ready),$(error glide dependency manager missing, info about installation can be found here https://github.com/Masterminds/glide))
	@echo "# updating dependencies, please wait ..."
	@-cd vendor && rm -rf *
	@echo "# vendor dir cleared"
	@-rm -rf glide.lock
	@glide cc
	@echo "# glide cache cleared"
	@glide install --strip-vendor
endef

# build-only binaries
define build_only
        @go version
        @echo "# building the sfc controller with plugins"
        @go build -i -v ${LDFLAGS}
        @echo "# done"
endef

# build-only sfcdump
define build_sfcdump_only
	@echo "# building sfcdump"
	@cd cmd/sfcdump && go build -v
	@echo "# done"
endef

# install-only binaries
define install_only
        @echo "# installing sfc controller with plugins"
        @go install

        @echo "# installing sfcdump"
        @cd cmd/sfcdump && go install -v


        if test "$(ETCDV3_CONFIG)" != "" ; then \
        echo "# Installing '$(ETCD_CONFIG_FILE)' to '$(ETCDV3_CONFIG)''..."; \
                cp $(ETCD_CONFIG_FILE) $(ETCDV3_CONFIG); \
        fi

        if test "$(KAFKA_CONFIG)" != "" ; then \
        echo "# Installing '$(KAFKA_CONFIG_FILE)' to '$(KAFKA_CONFIG)''..."; \
                cp $(KAFKA_CONFIG_FILE) $(KAFKA_CONFIG); \
        fi

        if test "$(SFC_CONFIG)" != "" ; then \
        echo "# Installing '$(SFC_CONFIG_FILE)' to '$(SFC_CONFIG)''..."; \
                cp $(SFC_CONFIG_FILE) $(SFC_CONFIG); \
        fi

        @echo "# done"
endef

# build binaries
build:
	$(call build_only)

# install binaries
install:
	$(call install_only)

# install dependencies
install-dep:
	$(call install_dependencies)

# update dependencies
update-dep:
	$(call update_dependencies)

# generate structures
generate:
	$(call generate_sources)

# build & install
all:
	$(call build_only)
	$(call build_sfcdump_only)
	$(call install_only)

# run tests
test:
	@cd bin_api && go test -cover
#       @cd etcd && go test -cover

# print golint suggestions to stderr
golint:
	@./scripts/golint.sh

.PHONY: golint

# report suspicious constructs using go vet tool
govet:
	@./scripts/govet.sh

.PHONY: govet

# clean
clean:
	@echo "# cleaning up the plugin binary"
	@rm -f ${PLUGIN_BIN}
	@echo "# done"

.PHONY: clean build

.PHONY: clean install
