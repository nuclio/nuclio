#!/usr/bin/env sh

echo "Displaying System Stats"
echo "-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-"
echo "## Memory ##"
echo "$ free -h"
free -h
echo "--------------------------"
echo "$ ps aux --sort=-%mem | head -n 20"
ps aux --sort=-%mem | head -n 20
echo "--------------------------"
echo "$ top -b -n 1 | head -n 20"
top -b -n 1 | head -n 20
echo "-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-"
echo "## CPU ##"
echo "$ lscpu"
lscpu
echo "--------------------------"
echo "$ ps aux | sort -k 3 -r | head -n 20"
ps aux | sort -k 3 -r | head -n 20
echo "--------------------------"
echo "$ mpstat -P ALL"
mpstat -P ALL
echo "-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-"
echo "## Disk ##"
echo "$ df --human-readable"
df --human-readable
echo "-----------"