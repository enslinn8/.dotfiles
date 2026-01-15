#!/bin/bash
monitor1=eDP-2
#monitor2=DP-2
#xrandr --output $monitor1 --primary --auto --mode 1920x1080 --rate 75
#xrandr --output $monitor2 --off
hyprctl keyword "monitor" "$monitor1, disable"
