package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func main() {
	serverHostname := "10.68.16.220"
	serverPort := 25565

	for {
		monitorServer(serverHostname, serverPort, 60*60, 60)
		//Be Server
	}
	return
}

//Server Monitor Code
func monitorServer(serverHostname string, serverPort int, threshold int, rate int) {
	secondsEmpty := 0
	fmt.Println(threshold, "::", secondsEmpty)
	for secondsEmpty <= threshold {
		if checkServerEmpty(serverHostname, serverPort) {
			secondsEmpty += rate
		} else {
			secondsEmpty = 0
		}
		time.Sleep(time.Duration(rate) * time.Second)
	}
	//Server Has been empty passed threshold
	cmd := exec.Command("bash", "-c", "systemctl stop minecraft")
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
		return
	}
}

func checkServerEmpty(serverHostname string, serverPort int) bool {
	con, err := connect(serverHostname, serverPort)
	if err != nil {
		log.Fatalln(err)
		return false
	}
	defer con.Close()
	handShakePacket := makePacket(makeHandshake(serverHostname, uint16(serverPort)))
	_, err = con.Write(handShakePacket)
	if err != nil {
		return false
	}
	statusPacket := makePacket(makeStatusPacket())
	fmt.Println(statusPacket)
	_, err = con.Write(statusPacket)
	if err != nil {
		return false
	}
	response := readStatusResponse(con)
	fmt.Println("Response:", response)
	if strings.Contains(response, "\"online\":0") {
		fmt.Println("Server appears to be empty")
		return true
	} else {
		return false
	}
}

func connect(serverHostname string, serverPort int) (net.Conn, error) {
	serverAddress := serverHostname + ":" + strconv.Itoa(serverPort)
	con, err := net.Dial("tcp", serverAddress)
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
	responseBuffer := make([]byte, responseLen)
	_, e = bufferReader.Read(responseBuffer)
	if e != nil {
		return ""
	}
	return string(responseBuffer[2:])
}

func makePacket(data []byte) []byte {
	var dataLen = uint64(len(data))
	var packetLen = make([]byte, 5)
	var packetLenLen = binary.PutUvarint(packetLen, dataLen) //Create varInt for packetLen
	var packet = make([]byte, packetLenLen+int(dataLen))
	copy(packet, packetLen)           //Copy packetLen to start of packet
	copy(packet[packetLenLen:], data) //copy data into packet
	return packet
}

func makeStatusPacket() []byte {
	var data = make([]byte, 1)
	data[0] = 0 //packet id
	return data
}

func makeHandshake(serverName string, port uint16) []byte {
	var handshakeBuffer = make([]byte, 256)
	var handshakeLen = binary.PutUvarint(handshakeBuffer, 0)              //Packet id 0x0
	handshakeLen += binary.PutVarint(handshakeBuffer[handshakeLen:], 756) //curent protocol
	handshakeLen += copy(handshakeBuffer[handshakeLen:], makeString(serverName))
	binary.BigEndian.PutUint16(handshakeBuffer[handshakeLen:], port)
	handshakeLen += 2
	handshakeLen += binary.PutUvarint(handshakeBuffer[handshakeLen:], 1) //set state to STATUS
	return handshakeBuffer[:handshakeLen]
}

func makeString(input string) []byte {
	var l = make([]byte, 5)
	var lLen = binary.PutUvarint(l, uint64(len(input)))
	var output = make([]byte, lLen+len(input))
	copy(output, l)
	copy(output[lLen:], input)
	return output
}
