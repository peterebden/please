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
RUN emerge -q dev-lang/go virtual/jdk, dev-libs/protobuf dev-go/golint

# Login message
COPY motd.txt /etc/motd
WORKDIR /tmp
