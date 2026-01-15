#!/bin/bash
monitor1=eDP-2
#xrandr --output $monitor2 --primary --auto
#xrandr --output $monitor1 --off

hyprctl keyword "monitor" "$monitor1, enable"

