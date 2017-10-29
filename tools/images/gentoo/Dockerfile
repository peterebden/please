FROM gentoo/stage3-amd64

# Base system & locales
COPY make.conf /etc/portage/make.conf
RUN emerge --sync -q
COPY locale.gen /etc/locale.gen
RUN locale-gen && eselect locale set en_GB.utf8 && env-update && source /etc/profile

# Python
RUN emerge -q python:3.5 net-misc/curl unzip dev-vcs/git
RUN emerge -q --newuse world

# Go and Java, protobufs, linter
# TODO(peterebden): Move this make.conf file up and upgrade portage earlier on.
COPY make.conf.2 /etc/portage/make.conf
# Unsure why sandbox breaks here?
RUN FEATURES="-sandbox -usersandbox" emerge -q dev-lang/go
RUN emerge -q virtual/jdk dev-libs/protobuf dev-go/golint

# TODO move this to a more logical spot too.
RUN emerge -q dev-python/cffi

WORKDIR /tmp
