package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
)

func main() {
	serverHostname := "10.68.16.220"
	serverPort := 25566

	con, err := connect(serverHostname,serverPort)
	if err != nil{
		log.Fatalln(err)
		return
	}
	defer con.Close()

	handShakePacket := makePacket(makeHandshake(serverHostname, uint16(serverPort)))
	fmt.Println(handShakePacket)
	_, err = con.Write(handShakePacket)
	if err != nil {
		return
	}

	statusPacket := makePacket(makeStatusPacket())
	fmt.Println(statusPacket)
	_, err = con.Write(statusPacket)
	if err != nil {
		return
	}

	response := readStatusResponse(con)
	fmt.Println("Response:",response)
	return
}

func connect(serverHostname string, serverPort int) (net.Conn, error) {
	serverAddress := serverHostname + ":" + strconv.Itoa(serverPort)
	con, err := net.Dial("tcp",serverAddress)
	if err != nil {
		log.Fatalln(err)
		return nil, errors.New("Unable to connect to: " + serverAddress)
	}
	return con, nil
}

func readStatusResponse(con net.Conn) string {
	//Read Response Len
	bufferReader := bufio.NewReader(con)
	responseLen, e := binary.ReadUvarint(bufferReader)
	if e != nil {
		return ""
	}
	//read Response
	responseBuffer := make([]byte,responseLen)
	_, e = bufferReader.Read(responseBuffer)
	if e != nil {
		return ""
	}
	return string(responseBuffer[2:])
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

func makeStatusPacket() []byte {
	var data = make([]byte,1)
	data[0] = 0 //packet id
	return data
}

func makeHandshake(serverName string, port uint16) []byte {
	var handshakeBuffer = make([]byte,256)
	var handshakeLen = binary.PutUvarint(handshakeBuffer,0) //Packet id 0x0
	handshakeLen += binary.PutVarint(handshakeBuffer[handshakeLen:],756) //curent protocol
	handshakeLen += copy(handshakeBuffer[handshakeLen:],makeString(serverName))
	binary.BigEndian.PutUint16(handshakeBuffer[handshakeLen:],port)
	handshakeLen += 2
	handshakeLen += binary.PutUvarint(handshakeBuffer[handshakeLen:],1) //set state to STATUS
	return handshakeBuffer[:handshakeLen]
}

func makeString(input string) []byte {
	var l = make([]byte,5)
	var lLen = binary.PutUvarint(l, uint64(len(input)))
	var output = make([]byte,lLen + len(input))
	copy(output,l)
	copy(output[lLen:],input)
	return output
}