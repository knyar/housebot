gst-launch-1.0 -q filesrc location=audio.raw ! audio/x-raw,format=S16LE,channels=1,rate=16000 ! autoaudiosink
