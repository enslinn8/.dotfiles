#!/bin/bash
monitor1=HDMI-0
monitor2=DP-2
xrandr --output $monitor1 --primary --auto
xrandr --output $monitor2 --off
