package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

//Hide the config data in a global variable... Tell no one of these sins...
var Config Configuration

func main() {
	Config = loadConfig()

	for {
		monitorServer(Config.Server, Config.Port, Config.EmptyThreshold, Config.CheckRate)
		beServer(Config.Port)
	}
}

func loadConfig() Configuration {
	fmt.Println(("Loading config.json"))
	file, _ := os.Open("config.json")
	defer file.Close()
	configuration := Configuration{}
	decoder := json.NewDecoder(file)
	err := decoder.Decode(&configuration)
	if err != nil {
		fmt.Println("Error:", err)
	}
	return configuration
}

//Server Monitor Code
func monitorServer(serverHostname string, serverPort int, threshold int, rate int) {
	fmt.Println("Waiting for minecraft server to load..")
	time.Sleep(time.Second * 120) //Wait for server to start
	secondsEmpty := 0
	fmt.Println(threshold, "::", secondsEmpty)
	for secondsEmpty <= threshold {
		result := checkServerEmpty(serverHostname, serverPort)
		if result == 0 {
			secondsEmpty += rate
		} else if result > 0 {
			fmt.Println("Server not empty, reset timer")
			secondsEmpty = 0
		} else if result == -1 {
			fmt.Println("Minecraft Server not started... Taking over")
			return
		} else {
			fmt.Println("Unimplemented error")
			return
		}
		time.Sleep(time.Second * time.Duration(rate))
	}
	//Server Has been empty passed threshold
	cmd := exec.Command("bash", "-c", Config.StopCommand)
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
		return
	}
}

func checkServerEmpty(serverHostname string, serverPort int) int {
	con, err := connect(serverHostname, serverPort)
	if err != nil {
		fmt.Println(err)
		return -1
	}
	defer con.Close()
	handShakePacket := makePacket(makeHandshake(serverHostname, uint16(serverPort)))
	_, err = con.Write(handShakePacket)
	if err != nil {
		return -5
	}
	statusPacket := makePacket(makeClientStatusPacket())
	fmt.Println(statusPacket)
	_, err = con.Write(statusPacket)
	if err != nil {
		return -5
	}
	response := readStatusResponse(con)
	fmt.Println("Response:", response)
	if strings.Contains(response, "\"online\":0") {
		fmt.Println("Server appears to be empty")
		return 0
	} else {
		return 1
	}
}

func connect(serverHostname string, serverPort int) (net.Conn, error) {
	serverAddress := serverHostname + ":" + strconv.Itoa(serverPort)
	con, err := net.Dial("tcp", serverAddress)
	if err != nil {
		fmt.Println(err)
		return nil, errors.New("Unable to connect to: " + serverAddress)
	}
	return con, nil
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

func makeClientStatusPacket() []byte {
	var data = make([]byte, 1)
	data[0] = 0 //packet id
	return data
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

//Server Pretend Core
func beServer(port int) bool {
	fmt.Println("Emulating minecraft server and waiting for client..")
	listenSocket, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		log.Fatalf("Unable to listen on port %d", port)
		return false
	}
	defer listenSocket.Close()
	done := make(chan string)
	go socketListen(listenSocket, done)
	for {
		select {
		case d := <-done:
			fmt.Println("Minecraft client connected...Start Server:", d)
			_ = listenSocket.Close()
			time.Sleep(time.Second * 3)
			cmd := exec.Command("bash", "-c", Config.StartCommand)
			err := cmd.Run()
			if err != nil {
				fmt.Println(err)
				return false
			}
			close(done)
			return true
		default:
			time.Sleep(time.Second)
		}
	}

}

func socketListen(listenSocket net.Listener, done chan string) {
	for {
		client, err := listenSocket.Accept()
		if err != nil {
			fmt.Println("Failed to Accept conneciton ", err)
			return
		}
		client.SetDeadline(time.Now().Add(10 * time.Second))
		go serverClientHandler(client, done)
	}
}

func serverClientHandler(client net.Conn, done chan string) {
	defer client.Close()
	startMinecraft := receiveHandhsake(client)
	if startMinecraft {
		fmt.Println("Client requesting to connect...Start server...")
		//TODO: Send stop signal via channel
		done <- "done"
	}
}

//Server Minecraft Protocol
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
	fmt.Println("Received protocol version:", protocolVersion)

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

func sendStatus(client net.Conn) {
	client.Write(makeStatusPacket())
}

func makeStatusPacket() []byte {
	statusString := "{\"version\":{\"protocol\":756,\"name\":\"Minecraft 1.17.1\"},\"players\":{\"online\":0,\"max\":" + Config.ServerMaxPlayers + ",\"sample\":[]},\"description\":{\"color\":\"dark_aqua\",\"text\":\"" + Config.ServerTitle + "\"}}"
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

func makePongPacket(payload []byte) []byte {
	pongPacket := make([]byte, 9)
	pongPacket[0] = 1
	copy(pongPacket[1:], payload)
	return makePacket(pongPacket)
}

func handleMinecraftClient(client net.Conn) {
	//A client is attempting to login.
	disconnectReason := []byte("{\"text\": \"" + Config.ClientError + "\"}")
	client.Write(makeDisconnectPacket(disconnectReason))
	client.Close()
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

//Util
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

func makePacket(data []byte) []byte {
	var dataLen = uint64(len(data))
	var packetLen = make([]byte, 5)
	var packetLenLen = binary.PutUvarint(packetLen, dataLen) //Create varInt for packetLen
	var packet = make([]byte, packetLenLen+int(dataLen))
	copy(packet, packetLen)           //Copy packetLen to start of packet
	copy(packet[packetLenLen:], data) //copy data into packet
	return packet
}

func makeString(input string) []byte {
	var l = make([]byte, 5)
	var lLen = binary.PutUvarint(l, uint64(len(input)))
	var output = make([]byte, lLen+len(input))
	copy(output, l)
	copy(output[lLen:], input)
	return output
}

type Configuration struct {
	Server           string `json:"server"`
	Port             int    `json:"port"`
	EmptyThreshold   int    `json:"emptyThreshold"`
	CheckRate        int    `json:"checkRate"`
	StartCommand     string `json:"startCommand"`
	StopCommand      string `json:"stopCommand"`
	ClientError      string `json:"clientError"`
	ServerTitle      string `json:"serverTitle"`
	ServerMaxPlayers string `json:"serverMaxPlayers"`
}
