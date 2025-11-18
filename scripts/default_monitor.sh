#!/bin/bash
monitor1=HDMI-0
monitor2=DP-2
xrandr --output $monitor2 --primary --auto
xrandr --output $monitor1 --off

