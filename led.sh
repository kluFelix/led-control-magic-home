#!/usr/bin/env bash

# CONFIGURATION
PORT=5577

# Helper: Send raw hex bytes using printf (most reliable)
send_raw() {
    local ip=$1
    shift
    local hex_string=""
    
    for byte in "$@"; do
        hex_string+=$(printf "\\x%02x" $((0x$byte)))
    done
    
    # Send via netcat with verbose output for debugging
    printf "%b" "$hex_string" | nc -v -w 1 "$ip" "$PORT" 2>&1
}

# COMMAND: Power ON
cmd_power_on() {
    echo "Sending Power On..."
    send_raw "$1" "71" "23" "0F"
    # Calculate checksum: 71 + 23 + 0F = 113 + 35 + 15 = 163 = 0xA3
    send_raw "$1" "71" "23" "0F" "A3"
}

# COMMAND: Power OFF  
cmd_power_off() {
    echo "Sending Power Off..."
    # Calculate checksum: 71 + 24 + 0F = 113 + 36 + 15 = 164 = 0xA4
    send_raw "$1" "71" "24" "0F" "A4"
}

# COMMAND: Set Color (RGB in Hex)
cmd_set_color() {
    local ip=$1
    local r=$2
    local g=$3
    local b=$4
    
    echo "Setting Color: R=$r G=$g B=$b"
    
    # Format: 31 [R] [G] [B] 00 0F 0F [Checksum]
    # Calculate checksum manually
    local sum=$((0x31 + 0x$r + 0x$g + 0x$b + 0x00 + 0x0F + 0x0F))
    local checksum=$(printf "%02x" $((sum & 0xFF)))
    
    send_raw "$ip" "31" "$r" "$g" "$b" "00" "0F" "0F" "$checksum"
}

# MAIN SCRIPT LOGIC
if [ $# -lt 2 ]; then
    echo "Usage: $0 <IP_ADDRESS> <COMMAND> [ARGS...]"
    echo "Commands:"
    echo "  on          - Turn lights on"
    echo "  off         - Turn lights off"
    echo "  color <R> <G> <B> - Set color (Hex values, e.g. FF 00 00)"
    exit 1
fi

IP=$1
CMD=$2
shift 2

case $CMD in
    on)
        cmd_power_on "$IP"
        ;;
    off)
        cmd_power_off "$IP"
        ;;
    color)
        if [ $# -lt 3 ]; then
            echo "Error: Color requires R G B arguments (e.g. FF 00 00)"
            exit 1
        fi
        cmd_set_color "$IP" "$1" "$2" "$3"
        ;;
    *)
        echo "Unknown command: $CMD"
        exit 1
        ;;
esac

echo "Done."
