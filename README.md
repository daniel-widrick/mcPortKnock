# Minecraft Port Knock

A simple program that performs two duties:
1. Monitor a Minecraft server and stop it after it has been empty for some amount of time
2. Emulate the Minecraft server, wait for a client connection and start the Minecraft server

## Purpose
An empty Minecraft server consumes a decent amount of resources. This program is designed to take advantage of downtime and periods of reduced activity to free up those resources. While in server emulation mode, the Minecraft server will appear to be online to Minecraft clients / websites / etc. 

If a client attempts to connect to the program while in emulation mode, the client will receive a configurable error and the Minecraft server will be started in the background.

## On Demand
The program essentially provides "on demand" access to a minecraft server that can be activated by a simple connect attempt from any Minecraft client.

## Usage
Update the settings in config.json and run `go run mcPortKnock.go`
Binary Releases coming Soon(tm)