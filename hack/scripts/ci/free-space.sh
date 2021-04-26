#!/usr/bin/env sh

print_free_space() {
  df --human-readable
}

# before cleanup
print_free_space

# clean unneeded os packages and misc
sudo apt-get remove --yes '^dotnet-.*' 'php.*' azure-cli google-cloud-sdk google-chrome-stable firefox powershell
sudo apt-get autoremove --yes
sudo apt clean

# cleanup unneeded share dirs ~30GB
sudo rm --recursive --force \
    /usr/local/lib/android \
    /usr/share/dotnet \
    /usr/share/miniconda \
    /usr/share/dotnet \
    /usr/share/swift

# clean unneeded docker images
docker system prune --all --force

# post cleanup
print_free_space
