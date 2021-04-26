ssh 10.11.12.128 "sudo gst-launch-1.0 -q alsasrc ! audioconvert ! audioresample ! audio/x-raw,format=S16LE,channels=1,rate=16000 ! filesink location=/dev/stdout" > audio.raw
