package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
)

func main() {
	serverHostname := "10.68.16.220"
	serverPort := 25566
	serverAddress := serverHostname + ":" + strconv.Itoa(serverPort)
	con, err := net.Dial("tcp",serverAddress)
	if err != nil {
		log.Fatalln(err)
		return
	}
	defer con.Close()


	//Send Handshake Packet
	var packetDataBuffer = make([]byte,1024)
	var packetDataLen = 0;
	packetDataLen += binary.PutUvarint(packetDataBuffer,0) // packet id
	packetDataLen += binary.PutUvarint(packetDataBuffer[packetDataLen:],756) //protocol id
	var serverNameBuffer = []byte(serverHostname)
	packetDataLen += binary.PutUvarint(packetDataBuffer[packetDataLen:],(uint64)(len(serverNameBuffer)))
	packetDataLen += copy(packetDataBuffer[packetDataLen:],serverNameBuffer)
	portNum := uint16(serverPort)
	binary.BigEndian.PutUint16(packetDataBuffer[packetDataLen:],portNum)
	packetDataLen += 2;
	packetDataLen  += binary.PutUvarint(packetDataBuffer[packetDataLen:],1) //1 status || 2 login

	var packetLenBuffer = make([]byte,5)
	var packetLenLen = binary.PutUvarint(packetLenBuffer,(uint64)(packetDataLen))

	var packetData = make([]byte,packetLenLen + packetDataLen)
	copy(packetData,packetLenBuffer)
	copy(packetData[packetLenLen:],packetDataBuffer)
	fmt.Println(packetData)
	con.Write(packetData) //Send Handshake Packet

	binary.PutUvarint(packetDataBuffer,1)
	binary.PutUvarint(packetDataBuffer[1:], 0)
	var statusPacket = make([]byte,2)
	copy(statusPacket,packetDataBuffer[:2])
	fmt.Println(statusPacket)
	con.Write(statusPacket)

	var response = make([]byte,200)
	read, err := con.Read(response)
	if err != nil {
		fmt.Println("Error:",err)
	}
	fmt.Println(read,response)
	var responseLen = int(response[0])
	fmt.Println(response[3:responseLen])
	fmt.Println(string(response[3:responseLen]))
	var responseString = string(response[3:responseLen])
	if strings.Contains(responseString,"\"online\":0") {
		fmt.Println("Server Seems Empty")
	}

}

func makePacket(data []byte) []byte {
	var dataLen = uint64(len(data))
	var packetLen = make([]byte,5)
	var packetLenLen = binary.PutUvarint(packetLen,dataLen) //Create varInt for packetLen
	var packet = make([]byte,packetLenLen + int(dataLen))
	copy(packet,packetLen) //Copy packetLen to start of packet
	copy(packet[packetLenLen:],data) //copy data into packet
	return packet
}

func sendHandshake() []byte {

}