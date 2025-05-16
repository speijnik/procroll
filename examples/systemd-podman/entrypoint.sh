#!/bin/sh
systemd-notify --ready
trap 'echo Received SIGTERM, finishing; exit' INT HUP QUIT TERM ALRM USR1
echo WAITING
while :
do 
  sleep 0.1
done
