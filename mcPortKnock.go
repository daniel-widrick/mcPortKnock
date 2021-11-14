package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
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
		beServer(serverPort)
		monitorServer(serverHostname, serverPort, 20, 10)
	}
	return
}

//Server Pretend Core
func beServer(port int) bool {
	listenSocket, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		log.Fatalf("Unable to listen on port %d", port)
		return false
	}
	defer listenSocket.Close()
	for {
		client, err := listenSocket.Accept()
		if err != nil {
			log.Fatalln(err)
			return false
		}
		client.SetDeadline(time.Now().Add(10 * time.Second)) //Clients have 3 seconds to get what they need and leave
		go serverClientHandler(client)                       //TODO: Channel to Exit loop from goroutine
	}
}

func serverClientHandler(client net.Conn) {
	defer client.Close()
	startMinecraft := receiveHandhsake(client)
	if startMinecraft {
		fmt.Println("Client requesting to connect...Start server...")
		//TODO: Send stop signal via channel
	}
}

func receiveHandhsake(client net.Conn) bool {
	//Receive Handshake
	handshakeBuffer, b := receivePacket(client)
	if b {
		return false //keep pretending to be minecraft
	}
	bufferReader := bytes.NewReader(handshakeBuffer)
	fmt.Println("Received Handshake:")

	//Packet Id
	packetId, err := binary.ReadUvarint(bufferReader)
	if err != nil {
		fmt.Println("Error reading packet id", err)
		return false
	}
	fmt.Println("Handshake contains packet id:", packetId)
	if packetId != 0 {
		fmt.Println("Wrong Packet ID for handshake received")
	}

	//Protocol Version
	protocolVersion, err := binary.ReadUvarint(bufferReader)
	fmt.Println("Received protocol version:")
	fmt.Println(protocolVersion)

	//Hostname
	hostnameLen, err := binary.ReadUvarint(bufferReader)
	if err != nil {
		fmt.Println("Error reading hostname len", err)
		return false
	}
	hostnameBytes := make([]byte, hostnameLen)
	_, err = io.ReadFull(bufferReader, hostnameBytes)
	if err != nil {
		fmt.Println("Error reading hostname", err)
		return false
	}
	hostname := string(hostnameBytes)
	fmt.Println("read hostname ", hostname)

	//Portnum
	portNumBytes := make([]byte, 2)
	_, err = io.ReadFull(bufferReader, portNumBytes)
	if err != nil {
		fmt.Println("Error reading port number", err)
		return false
	}
	portNum := binary.BigEndian.Uint16(portNumBytes)
	fmt.Println("Read portnum:", portNum)

	//Next State
	nextState, err := binary.ReadUvarint(bufferReader)
	if err != nil {
		fmt.Println("Error reading next state", err)
		return false
	}
	fmt.Println("Next State:", nextState)

	if nextState == 1 {
		fmt.Println("Received Next State: Status")
		receivePing(client)
		return false
	} else if nextState == 2 {
		fmt.Println("Received Next State: Login")
		handleMinecraftClient(client)
		return true //stop pretending to be minecraft
	}
	return false //keep pretending to be minecraft
}

func handleMinecraftClient(client net.Conn) {
	//A client is attempting to login.
	disconnectReason := []byte("{\"text\": \"Server Paused... Starting Now! Please reconnect in 2 minutes\"}")
	client.Write(makeDisconnectPacket(disconnectReason))
	client.Close()
}

func sendStatus(client net.Conn) {
	client.Write(makeStatusPacket())
}

func makePongPacket(payload []byte) []byte {
	pongPacket := make([]byte, 9)
	pongPacket[0] = 1
	copy(pongPacket[1:], payload)
	return makePacket(pongPacket)
}
func receivePing(con net.Conn) {
	pingPacket, err := receivePacket(con)
	if err {
		fmt.Println("Error Receiving ping!")
		return
	}
	packetId, packetIdLen := binary.Uvarint(pingPacket)
	if packetId == 0 {
		//Client is Requesting status
		sendStatus(con)
		receivePing(con) //Recurse for ping
	} else if packetId == 1 {
		//Client is requesting ping
		payloadBytes := pingPacket[packetIdLen:8]
		con.Write(makePongPacket(payloadBytes))
		con.Close()
	} else {
		fmt.Printf("Received unexpected packet id: %d. Expected 1 for ping\n", packetId)
		fmt.Println(pingPacket)
	}
}

func receivePacket(con net.Conn) ([]byte, bool) {
	bufferReader := bufio.NewReader(con)
	responseLen, err := binary.ReadUvarint(bufferReader)
	if err != nil {
		fmt.Println("Error reading length of packet")
		fmt.Println(err)
		return make([]byte, 0), true
	}
	fmt.Printf("Reading %d bytes\n", responseLen)
	buffer := make([]byte, responseLen)
	i, err := io.ReadFull(bufferReader, buffer)
	if err != nil {
		fmt.Println("Error reading packet")
		fmt.Println(err)
		return make([]byte, 0), true
	}
	fmt.Printf("read %d bytes\n", i)
	return buffer, false
}

func readBytes(con net.Conn, length int) []byte {
	readCount := 0
	buffer := make([]byte, length)
	for readCount < length {
		i, err := con.Read(buffer[readCount:])
		if err != nil {
			return make([]byte, 0)
		}
		readCount += i
		fmt.Printf("read %d of %d\n", readCount, length)
	}
	return buffer
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
	statusPacket := makePacket(makeClientStatusPacket())
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

func makeStatusPacket() []byte {
	statusString := "{\"version\":{\"protocol\":756,\"name\":\"Minecraft 1.17.1\"},\"players\":{\"online\":0,\"max\":500,\"sample\":[]},\"description\":{\"color\":\"dark_aqua\",\"text\":\"A 315Gaming Server\"}}"
	statusBytes := []byte(statusString)
	statusBytesVarint := make([]byte, 5)
	dataLen := uint64(len(statusBytes))
	statusBytesVarintLen := binary.PutUvarint(statusBytesVarint, dataLen)
	statusPacket := make([]byte, statusBytesVarintLen+len(statusBytes)+1)
	statusPacket[0] = 0 //Packet ID
	offset := 1
	copy(statusPacket[offset:], statusBytesVarint[:statusBytesVarintLen])
	offset += statusBytesVarintLen
	copy(statusPacket[offset:], statusBytes)
	return makePacket(statusPacket)
}

func makeDisconnectPacket(reason []byte) []byte {
	reasonVarint := make([]byte, 3)
	reasonVarintLen := binary.PutUvarint(reasonVarint, uint64(len(reason)))
	disconnectPacket := make([]byte, len(reason)+reasonVarintLen+1)
	disconnectPacket[0] = 0 //Packet ID
	offset := 1
	copy(disconnectPacket[offset:], reasonVarint[:reasonVarintLen])
	offset += reasonVarintLen
	copy(disconnectPacket[offset:], reason)
	packetBytes := makePacket(disconnectPacket)
	return packetBytes
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

func makeClientStatusPacket() []byte {
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
