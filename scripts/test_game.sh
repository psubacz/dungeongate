#!/bin/bash
# Test script to connect to DungeonGate and try to play a game

echo "Testing DungeonGate game connection..."
echo "This will connect via SSH and attempt to play NetHack."
echo ""
echo "Instructions:"
echo "1. Login with username: yellow, password: yellowdog"
echo "2. Select option 1 to play NetHack"
echo "3. Check if the game loads properly"
echo ""
echo "Press Enter to continue..."
read

# Connect to SSH
ssh -p 2222 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null localhost