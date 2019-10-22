Name: dns-server
Version: 0.0.7
Release: 1
Summary: DNS Server
License: FIXME

# disable facist builds, we dont care about files we arent installing
%define _unpackaged_files_terminate_build 0

%description
a bad description for an awesome package

%prep

%build
go build -o dns-server

%install
mkdir -p %{buildroot}/usr/local/bin
mkdir -p %{buildroot}/etc/systemd/system
install -m 755 dns-server %{buildroot}/usr/local/bin/dns-server
install -m 755 dns-server.service %{buildroot}/etc/systemd/system/dns-server.service

%files
/usr/local/bin/dns-server
/etc/systemd/system/dns-server.service

%changelog
# We will revisit
