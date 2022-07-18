package database

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ratel-online/core/log"
	"github.com/ratel-online/core/model"
	"github.com/ratel-online/core/network"
	"github.com/ratel-online/core/protocol"
	"github.com/ratel-online/core/util/arrays"
	"github.com/ratel-online/core/util/json"
	"github.com/ratel-online/core/util/poker"
	"github.com/ratel-online/server/consts"
	mjCard "github.com/ratel-online/server/mahjong/card"
	mjConsts "github.com/ratel-online/server/mahjong/consts"
	mjEvent "github.com/ratel-online/server/mahjong/event"
	mjGame "github.com/ratel-online/server/mahjong/game"
	"github.com/ratel-online/server/mahjong/tile"
	"github.com/ratel-online/server/uno/card"
	"github.com/ratel-online/server/uno/card/color"
	"github.com/ratel-online/server/uno/event"
	"github.com/ratel-online/server/uno/game"
)

const initialRune = 'A'

type runeSequence struct {
	currentRune rune
}

func (s *runeSequence) next() rune {
	if s.currentRune == 0 {
		s.currentRune = initialRune
	}
	currentRune := s.currentRune
	s.currentRune++
	return currentRune
}

type Player struct {
	ID     int64  `json:"id"`
	IP     string `json:"ip"`
	Name   string `json:"name"`
	Score  int64  `json:"score"`
	Mode   int    `json:"mode"`
	Type   int    `json:"type"`
	RoomID int64  `json:"roomId"`

	conn   *network.Conn
	data   chan *protocol.Packet
	read   bool
	state  consts.StateID
	online bool
}

func (p *Player) MahjongPlayer() mjGame.Player {
	return p
}

func (p *Player) PlayPrivileges(tiles []int, gameState mjGame.State) (int, []int, error) {
	p = getPlayer(p.ID)
	Broadcast(p.RoomID, fmt.Sprintf("It's %s turn! \n", p.Name), p.ID)
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("It's your turn, %s! \n", p.Name))
	buf.WriteString(gameState.String())
	p.WriteString(buf.String())
	askBuf := bytes.Buffer{}
	tileOptions := make(map[string][]int)
	runeSequence := runeSequence{}
	ret := 0
	if pv, ok := gameState.SpecialPrivileges[p.ID]; ok {
		label := string(runeSequence.next())
		if pv == mjConsts.GANG {
			askBuf.WriteString("You can 杠!!!\n")
			ret = mjConsts.GANG
			tileOptions[label] = []int{gameState.LastPlayedTile, gameState.LastPlayedTile, gameState.LastPlayedTile, gameState.LastPlayedTile}
		}
		if pv == mjConsts.PENG {
			askBuf.WriteString("You can 碰!!!\n")
			ret = mjConsts.PENG
			tileOptions[label] = []int{gameState.LastPlayedTile, gameState.LastPlayedTile, gameState.LastPlayedTile}
		}
		askBuf.WriteString(fmt.Sprintf("%s:%s \n", label, "yes"))
	} else {
		askBuf.WriteString("You can 吃!!!\n")
		ret = mjConsts.CHI
		for _, ts := range mjCard.CanChiTiles(tiles, gameState.LastPlayedTile) {
			label := string(runeSequence.next())
			tileOptions[label] = append(ts, gameState.LastPlayedTile)
			askBuf.WriteString(fmt.Sprintf("%s:%s \n", label, tile.ToTileString(ts)))
		}
	}
	label := string(runeSequence.next())
	tileOptions[label] = []int{}
	askBuf.WriteString(fmt.Sprintf("%s:%s \n", label, "no"))
	for {
		p = getPlayer(p.ID)
		p.WriteString(askBuf.String())
		selectedLabel, err := p.AskForString(consts.PlayMahjongTimeout)
		if err != nil {
			if err == consts.ErrorsTimeout {
				selectedLabel = "A"
			} else {
				return ret, nil, err
			}
		}
		selectedCards, found := tileOptions[strings.ToUpper(selectedLabel)]
		if !found {
			BroadcastChat(p, fmt.Sprintf("%s say: %s\n", p.Name, selectedLabel))
			continue
		}
		return ret, selectedCards, nil
	}
}
func (p *Player) PlayMJ(tiles []int, gameState mjGame.State) (int, error) {
	p = getPlayer(p.ID)
	Broadcast(p.RoomID, fmt.Sprintf("It's %s turn! \n", p.Name), p.ID)
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("It's your turn, %s! \n", p.Name))
	buf.WriteString(gameState.String())
	p.WriteString(buf.String())
	askBuf := bytes.Buffer{}
	askBuf.WriteString("Select a tile to play:\n")
	runeSequence := runeSequence{}
	tileOptions := make(map[string]int)
	for _, i := range tiles {
		label := string(runeSequence.next())
		tileOptions[label] = i
		askBuf.WriteString(fmt.Sprintf("%s:%s ", label, tile.Tile(i).String()))
	}
	askBuf.WriteString("\n")
	for {
		p = getPlayer(p.ID)
		p.WriteString(askBuf.String())
		selectedLabel, err := p.AskForString(consts.PlayMahjongTimeout)
		if err != nil {
			if err == consts.ErrorsTimeout {
				selectedLabel = "A"
			} else {
				return 0, err
			}
		}
		selectedCard, found := tileOptions[strings.ToUpper(selectedLabel)]
		if !found {
			BroadcastChat(p, fmt.Sprintf("%s say: %s\n", p.Name, selectedLabel))
			continue
		}
		p.OnPlayTile(mjEvent.PlayTilePayload{
			PlayerName: p.Name,
			Tile:       selectedCard,
		})
		return selectedCard, nil
	}
}

func (p *Player) NotifyTilesDrawn(drawnTiles []int) {
	p = getPlayer(p.ID)
	getPlayer(p.ID).WriteString(fmt.Sprintf("You drew %s!\n", tile.ToTileString(drawnTiles)))
}

func (p *Player) GamePlayer() game.Player {
	return p
}

func (p *Player) PlayerID() int64 {
	return p.ID
}

func (p *Player) NickName() string {
	return p.Name
}

func (p *Player) PickColor(gameState game.State) color.Color {
	for {
		p = getPlayer(p.ID)
		p.WriteString(fmt.Sprintf(
			"Select a color: '%s', '%s', '%s' or '%s'? \n",
			color.Red,
			color.Yellow,
			color.Green,
			color.Blue,
		))
		colorName, err := p.AskForString(consts.PlayTimeout)
		if err != nil {
			if err == consts.ErrorsTimeout {
				return color.Red
			}
			p.WriteString(fmt.Sprintf("Unknown color '%s' \n", colorName))
			continue
		}
		chosenColor, err := color.ByName(strings.ToLower(colorName))
		if err != nil {
			p.WriteString(fmt.Sprintf("Unknown color '%s' \n", colorName))
			continue
		}
		return chosenColor
	}
}

func (p *Player) Play(playableCards []card.Card, gameState game.State) (card.Card, error) {
	p = getPlayer(p.ID)
	Broadcast(p.RoomID, fmt.Sprintf("It's %s turn! \n", p.Name), p.ID)
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("It's your turn, %s! \n", p.Name))
	buf.WriteString(gameState.String())
	p.WriteString(buf.String())
	runeSequence := runeSequence{}
	cardOptions := make(map[string]card.Card)
	for _, card := range playableCards {
		label := string(runeSequence.next())
		cardOptions[label] = card
	}
	cardSelectionLines := []string{"Select a card to play:"}
	for label, card := range cardOptions {
		cardSelectionLines = append(cardSelectionLines, fmt.Sprintf("%s %s", card, label))
	}
	cardSelectionMessage := strings.Join(cardSelectionLines, " \n ") + " \n "
	for {
		p = getPlayer(p.ID)
		p.WriteString(cardSelectionMessage)
		selectedLabel, err := p.AskForString(consts.PlayTimeout)
		if err != nil {
			if err == consts.ErrorsTimeout {
				selectedLabel = "A"
			} else {
				return nil, err
			}
		}
		selectedCard, found := cardOptions[strings.ToUpper(selectedLabel)]
		if !found {
			BroadcastChat(p, fmt.Sprintf("%s say: %s\n", p.Name, selectedLabel))
			continue
		}
		if !contains(playableCards, selectedCard) {
			p.WriteString(fmt.Sprintf("Cheat detected! Card %s is not in %s's hand! \n", selectedCard, p.NickName()))
			continue
		}
		return selectedCard, nil
	}
}

func contains(cards []card.Card, searchedCard card.Card) bool {
	for _, card := range cards {
		if card.Equal(searchedCard) {
			return true
		}
	}
	return false
}

func (p *Player) OnPlayTile(payload mjEvent.PlayTilePayload) {
	p = getPlayer(p.ID)
	p.WriteString(fmt.Sprintf("You play %s ! \n", tile.Tile(payload.Tile)))
	Broadcast(p.RoomID, fmt.Sprintf("%s PlayTile %s !\n", payload.PlayerName, tile.Tile(payload.Tile)), p.ID)
}

func (p *Player) OnFirstCardPlayed(payload event.FirstCardPlayedPayload) {
	p = getPlayer(p.ID)
	Broadcast(p.RoomID, fmt.Sprintf("First card is %s\n", payload.Card))
}

func (p *Player) OnCardPlayed(payload event.CardPlayedPayload) {
	p = getPlayer(p.ID)
	Broadcast(p.RoomID, fmt.Sprintf("%s played %s!\n", payload.PlayerName, payload.Card))
}

func (p *Player) OnColorPicked(payload event.ColorPickedPayload) {
	p = getPlayer(p.ID)
	Broadcast(p.RoomID, fmt.Sprintf("%s picked color %s!\n", payload.PlayerName, payload.Color))
}

func (p *Player) OnPlayerPassed(payload event.PlayerPassedPayload) {
	p = getPlayer(p.ID)
	Broadcast(p.RoomID, fmt.Sprintf("%s passed!\n", payload.PlayerName))
}

func (p *Player) NotifyCardsDrawn(cards []card.Card) {
	p = getPlayer(p.ID)
	getPlayer(p.ID).WriteString(fmt.Sprintf("You drew %s!\n", cards))
}

func (p *Player) NotifyNoMatchingCardsInHand(lastPlayedCard card.Card, hand []card.Card) {
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("%s, none of your cards match %s! \n", p.Name, lastPlayedCard))
	buf.WriteString(fmt.Sprintf("Your hand is %s \n", hand))
	getPlayer(p.ID).WriteString(buf.String())
}

func (p *Player) Write(bytes []byte) error {
	return p.conn.Write(protocol.Packet{
		Body: bytes,
	})
}

func (p *Player) Offline() {
	p.online = false
	_ = p.conn.Close()
	close(p.data)
	room := getRoom(p.RoomID)
	if room != nil {
		room.Lock()
		defer room.Unlock()
		broadcast(room, fmt.Sprintf("%s lost connection! \n", p.Name))
		if room.State == consts.RoomStateWaiting {
			leaveRoom(room, p)
		}
		roomCancel(room)
	}
}

func (p *Player) Listening() error {
	for {
		pack, err := p.conn.Read()
		if err != nil {
			log.Error(err)
			return err
		}
		if p.read {
			p.data <- pack
		}
	}
}

// 向客户端发生消息
func (p *Player) WriteString(data string) error {
	time.Sleep(30 * time.Millisecond)
	return p.conn.Write(protocol.Packet{
		Body: []byte(data),
	})
}

func (p *Player) WriteObject(data interface{}) error {
	return p.conn.Write(protocol.Packet{
		Body: json.Marshal(data),
	})
}

func (p *Player) WriteError(err error) error {
	if err == consts.ErrorsExist {
		return err
	}
	return p.conn.Write(protocol.Packet{
		Body: []byte(err.Error() + "\n"),
	})
}

func (p *Player) AskForPacket(timeout ...time.Duration) (*protocol.Packet, error) {
	p.StartTransaction()
	defer p.StopTransaction()
	return p.askForPacket(timeout...)
}

func (p *Player) askForPacket(timeout ...time.Duration) (*protocol.Packet, error) {
	var packet *protocol.Packet
	if len(timeout) > 0 {
		select {
		case packet = <-p.data:
		case <-time.After(timeout[0]):
			return nil, consts.ErrorsTimeout
		}
	} else {
		packet = <-p.data
	}
	if packet == nil {
		return nil, consts.ErrorsChanClosed
	}
	single := strings.ToLower(packet.String())
	if single == "exit" || single == "e" {
		return nil, consts.ErrorsExist
	}
	return packet, nil
}

func (p *Player) AskForInt(timeout ...time.Duration) (int, error) {
	packet, err := p.AskForPacket(timeout...)
	if err != nil {
		return 0, err
	}
	return packet.Int()
}

func (p *Player) AskForInt64(timeout ...time.Duration) (int64, error) {
	packet, err := p.AskForPacket(timeout...)
	if err != nil {
		return 0, err
	}
	return packet.Int64()
}

func (p *Player) AskForString(timeout ...time.Duration) (string, error) {
	packet, err := p.AskForPacket(timeout...)
	if err != nil {
		return "", err
	}
	return packet.String(), nil
}

func (p *Player) AskForStringWithoutTransaction(timeout ...time.Duration) (string, error) {
	packet, err := p.askForPacket(timeout...)
	if err != nil {
		return "", err
	}
	return packet.String(), nil
}

func (p *Player) StartTransaction() {
	p.read = true
	_ = p.WriteString(consts.IsStart)
}

func (p *Player) StopTransaction() {
	p.read = false
	_ = p.WriteString(consts.IsStop)
}

func (p *Player) State(s consts.StateID) {
	p.state = s
}

func (p *Player) GetState() consts.StateID {
	return p.state
}

func (p *Player) Conn(conn *network.Conn) {
	p.conn = conn
	p.data = make(chan *protocol.Packet, 8)
	p.online = true
}

func (p Player) Model() model.Player {
	modelPlayer := model.Player{
		ID:    p.ID,
		Name:  p.Name,
		Score: p.Score,
	}
	room := getRoom(p.RoomID)
	if room != nil && room.Game != nil {
		modelPlayer.Pokers = len(room.Game.Pokers[p.ID])
		modelPlayer.Group = room.Game.Groups[p.ID]
	}
	return modelPlayer
}

func (p Player) String() string {
	return fmt.Sprintf("%s[%d]", p.Name, p.ID)
}

type Room struct {
	sync.Mutex

	ID                int64     `json:"id"`
	Type              int       `json:"type"`
	Game              *Game     `json:"gameId"`
	UnoGame           *UnoGame  `json:"unoGame"`
	Mahjong           *Mahjong  `json:"mahjong"`
	State             int       `json:"state"`
	Players           int       `json:"players"`
	Robots            int       `json:"robots"`
	Creator           int64     `json:"creator"`
	ActiveTime        time.Time `json:"activeTime"`
	MaxPlayers        int       `json:"maxPlayers"`
	Password          string    `json:"password"`
	EnableChat        bool      `json:"enableChat"`
	EnableLaiZi       bool      `json:"enableLaiZi"`
	EnableSkill       bool      `json:"enableSkill"`
	EnableLandlord    bool      `json:"enableLandlord"`
	EnableDontShuffle bool      `json:"enableDontShuffle"`
	Banker            int64     `json:"banker"`
}

func (r *Room) Model() model.Room {
	return model.Room{
		ID:        r.ID,
		Type:      r.Type,
		TypeDesc:  consts.GameTypes[r.Type],
		Players:   r.Players,
		State:     r.State,
		StateDesc: consts.RoomStates[r.State],
		Creator:   r.Creator,
	}
}

type Game struct {
	Room        *Room                   `json:"room"`
	Players     []int64                 `json:"players"`
	Groups      map[int64]int           `json:"groups"`
	States      map[int64]chan int      `json:"states"`
	Pokers      map[int64]model.Pokers  `json:"pokers"`
	Universals  []int                   `json:"universals"`
	Decks       int                     `json:"decks"`
	Additional  model.Pokers            `json:"pocket"`
	Multiple    int                     `json:"multiple"`
	FirstPlayer int64                   `json:"firstPlayer"`
	LastPlayer  int64                   `json:"lastPlayer"`
	Robs        []int64                 `json:"robs"`
	FirstRob    int64                   `json:"firstRob"`
	LastRob     int64                   `json:"lastRob"`
	FinalRob    bool                    `json:"finalRob"`
	LastFaces   *model.Faces            `json:"lastFaces"`
	LastPokers  model.Pokers            `json:"lastPokers"`
	Mnemonic    map[int]int             `json:"mnemonic"`
	Skills      map[int64]int           `json:"skills"`
	PlayTimes   map[int64]int           `json:"playTimes"`
	PlayTimeOut map[int64]time.Duration `json:"playTimeOut"`
	Rules       poker.Rules             `json:"rules"`
	Discards    model.Pokers            `json:"discards"`
}

func (g Game) NextPlayer(curr int64) int64 {
	idx := arrays.IndexOf(g.Players, curr)
	return g.Players[(idx+1)%len(g.Players)]
}

func (g Game) PrevPlayer(curr int64) int64 {
	idx := arrays.IndexOf(g.Players, curr)
	return g.Players[(idx+len(g.Players))%len(g.Players)]
}

func (g Game) IsTeammate(player1, player2 int64) bool {
	return g.Groups[player1] == g.Groups[player2]
}

func (g Game) IsLandlord(playerId int64) bool {
	return g.Groups[playerId] == 1
}

func (g Game) Team(playerId int64) string {
	if !g.Room.EnableLandlord {
		return "team" + strconv.Itoa(g.Groups[playerId])
	} else {
		if !g.IsLandlord(playerId) {
			return "peasant"
		} else {
			return "landlord"
		}
	}
}

type UnoGame struct {
	Room         *Room              `json:"room"`
	Players      []int64            `json:"players"`
	States       map[int64]chan int `json:"states"`
	Game         *game.Game         `json:"game"`
	PlayerNumber int                `json:"playerNumber"`
}

func (ug *UnoGame) HavePlay(player *Player) bool {
	for _, id := range ug.Players {
		if id == player.ID && player.online {
			return true
		}
	}
	return false
}

func (un *UnoGame) NeedExit() bool {
	return un.PlayerNumber <= 1
}

type Mahjong struct {
	Room    *Room              `json:"room"`
	Players []int64            `json:"players"`
	States  map[int64]chan int `json:"states"`
	Game    *mjGame.Game       `json:"game"`
}
