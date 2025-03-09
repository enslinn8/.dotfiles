#!/bin/bash

if grep -q "HDMI-0 connected" <<< `xrandr`; then
	source ~/scripts/change_monitor.sh
fi

