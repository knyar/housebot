#!/bin/bash

ssh 10.11.12.128 "cat /home/ryzh/housebot/recording/$1" | gst-launch-1.0 filesrc location=/dev/stdin ! audio/x-raw,format=S16LE,channels=1,rate=16000 ! autoaudiosink
