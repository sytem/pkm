package obs

import (
	"encoding/json"

	"log"
	"net"
	"net/url"
	"os"
	"strings"

	. "code.yensid.fi/tools"
	"github.com/jmoiron/jsonq"
	"strconv"

	"github.com/gorilla/websocket"
)

type (
	casparConfig struct {
		address  string
		amcpPort string
		oscPort  string
		out      string
		amcpConn net.Conn
	}

	Player struct {
		Server  int    `json:"server"`
		Channel string `json:"channel"`
	}

	Server struct {
		VMixInput int64 `json:"vmix-input"`
		Observer  int64 `json:"observer"`
	}

	ConfigFile struct {
		Servers  []Server          `json:"servers"`
		Players  map[string]Player `json:"players"`
		Commands map[string]string `json:"commands"`
	}

	obsConfig  struct {
		address  string
		port string
		conn *websocket.Conn
	}

	// OBS:lle lähetettävä komento
	SetSceneItemRender struct {
		RequestType string `json:"request-type"`
		MessageId string `json:"message-id"`
		Source	string	 `json:"source"`
		Render	bool  		`json:"render"`
		SceneName	string 	`json:"scene-name"`
	}
)

var (
	obs						 []obsConfig
	commands       map[string]string
	Players        map[string]Player
	Servers        map[int64]int64 // vMix -> Caspar mapping
	previousPlayer string
	previousInput  int
	messageID 		 int
)

func Configure() {

	obs = make([]obsConfig, 2)

	obs[0].address = GetEnvParam("OBS_0_ADDRESS", "127.0.0.1")
	obs[0].port = GetEnvParam("OBS_0_PORT", "4444")

	obs[1].address = GetEnvParam("OBS_1_ADDRESS", "127.0.0.1")
	obs[1].port = GetEnvParam("OBS_1_PORT", "4444")

	conffile := ConfigFile{}
	confFilename := GetEnvParam("CASPAR_CONFIG", "pkm.json")
	readConfig(&conffile, confFilename)

	Servers = make(map[int64]int64)
	for _, v := range conffile.Servers {
		Servers[v.VMixInput] = v.Observer
	}

	Players = make(map[string]Player)
	Players = conffile.Players

	commands = make(map[string]string)
	commands = conffile.Commands

	messageID = 0

	connectOBS(obs[0].address, obs[0].port, 0)
	connectOBS(obs[1].address, obs[1].port, 1)
}

func PopulatePlayerConf(jsonData string) {
	plrs := make(map[string]Player)

	// Jos player conffi on tyhjä, ota allplayers tieto pelidatasta ja laita niistä SteamID:t talteen
	testing := map[string]interface{}{}
	dec := json.NewDecoder(strings.NewReader(jsonData))
	dec.Decode(&testing)
	jq := jsonq.NewQuery(testing)
	obj, _ := jq.Object("allplayers")

	var n int = 1
	for k := range obj {
		plr := Player{}
		plr.Server = Players[strconv.Itoa(n)].Server
		plr.Channel = Players[strconv.Itoa(n)].Channel
		plrs[k] = plr
		n++
	}
}

// SwitchPlayer käskee tunnettuja palvelimia vaihtamaan inputtia, samat komennot jokaiselle.
//Inputtien nimet pitää olla OBS:ssä uniikkeja jotta vain oikea kone reagoi (muut antavat virheen josta ei välitetä)
func SwitchPlayer(input int64, currentPlayer string) {
  //ei taideta käyttää ASM-S18
	if Servers[input] == 0 {
		return
	}

	log.Printf("Observattava pelaaja vaihtui %d -> %d", previousPlayer, currentPlayer)
	if Players[currentPlayer].Channel == "" {
		log.Printf("Pelaajatunnusta %s ei löytynyt. Pelaajakuvan vaihto ei onnistu.", currentPlayer)
		return
	}


	sendCommand(Players[currentPlayer].Channel, true, 0)
	previousPlayer = currentPlayer
}

func sendCommand(input string, vis bool, server int) {

	messageID++

	commandToSend := &SetSceneItemRender{
		RequestType: "SetSceneItemRender",
		MessageId: strconv.Itoa(messageID),
		Source: input, // cam1..cam10
		Render: vis,
		SceneName: "Scene1"}

	jsonToSend, _ := json.Marshal(commandToSend)


	err := obs[server].conn.WriteMessage(websocket.TextMessage, jsonToSend)
	if err != nil {
		log.Println("write:", err)
		return
	}

}

func connectOBS(address string, port string, server int) {
	var err error
	addr := address + ":" + port
	u := url.URL{Scheme: "ws", Host: addr, Path: "/"}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	obs[server].conn = c
	if err != nil {
		log.Printf("Yhteys OBS-palvelimeen %s:%s epäonnistui: %s", address, port, err)
	}
	log.Printf("Yhteys OBS-palvelimeen %s:%s avattu", address, port)
}

func readConfig(conf *ConfigFile, filename string) {
	file, _ := os.Open(filename)
	decoder := json.NewDecoder(file)
	err := decoder.Decode(conf)
	if err != nil {
		log.Fatal("Konfiguraatiotiedoston lukuvirhe: ", err)
	}
}