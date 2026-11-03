package nucleo

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/crypto/nacl/secretbox"
)

/* ---------- the official network (protocol/official_network.dart) ---------- */

const (
	Fundador      = "704f0424bbde3f368c0bce7bd28d7a7c7f9ae88e589d9c51a0a2505d31be121a"
	Genesis       = "G1 net=zonda founder=704f0424bbde3f368c0bce7bd28d7a7c7f9ae88e589d9c51a0a2505d31be121a t=1793647553894 sig=e1d7700102269e3806092c4139f5ee9a2765bca2a10d35ce2c7557adc8375705d28ecc9c67a557481da85a9233c7d77ba9e999adaafaa14da177fd2d09b74604"
	PuertaAbierta = "OD1 net=zonda by=704f0424bbde3f368c0bce7bd28d7a7c7f9ae88e589d9c51a0a2505d31be121a exp=1801423562073 zeros=4 sig=4fac3632a3cc845d1c87285f5639bbf8fca36ec1dd35b789225cf2bde89bf67842806591f670337e754442d7cb32757fbe2b18e10ef733b951405fd12f48580f"
)

/* ---------- membership: certificates and the founder's open door (protocol/enrollment.dart) ---------- */

var (
	rxCert    = regexp.MustCompile(`^C1 m=([0-9a-f]{64}) by=([0-9a-f]{64}) sig=([0-9a-f]{128})$`)
	rxPuerta  = regexp.MustCompile(`^OD1 net=(\S+) by=([0-9a-f]{64}) exp=(\d+) zeros=(\d+) sig=([0-9a-f]{128})$`)
	rxEntrada = regexp.MustCompile(`^C2 m=([0-9a-f]{64}) door=([0-9a-f]{16}) dev=([0-9a-f]{16}) t=(\d+) n=(\d+) sig=([0-9a-f]{128})$`)
)

type puerta struct {
	by    string
	exp   int64
	zeros int
}

func verificarPuerta(door string) *puerta {
	m := rxPuerta.FindStringSubmatch(strings.TrimSpace(door))
	if m == nil || !Verificar(m[2], fmt.Sprintf("OD1 net=%s by=%s exp=%s zeros=%s", m[1], m[2], m[3], m[4]), m[5]) {
		return nil
	}
	exp, _ := strconv.ParseInt(m[3], 10, 64)
	z, _ := strconv.Atoi(m[4])
	return &puerta{m[2], exp, z}
}

func idPuerta(door string) string { return Hash(strings.TrimSpace(door))[:16] }

func peajeOk(door, key string, nonce int64, zeros int) bool {
	return strings.HasPrefix(Hash(fmt.Sprintf("ODW|%s|%s|%d", idPuerta(door), key, nonce)), strings.Repeat("0", zeros))
}

// Peaje searches the door's proof of work for key (OpenDoor.work).
func Peaje(door, key string) int64 {
	d := verificarPuerta(door)
	if d == nil {
		return -1
	}
	for n := int64(0); ; n++ {
		if peajeOk(door, key, n, d.zeros) {
			return n
		}
	}
}

// Entrar signs this identity's entry through the door (OpenDoor.enter).
func Entrar(me *Identidad, door string, nonce, t int64, device string) string {
	dev := Hash("ODD|" + idPuerta(door) + "|" + device)[:16]
	body := fmt.Sprintf("C2 m=%s door=%s dev=%s t=%d n=%d", me.Publica(), idPuerta(door), dev, t, nonce)
	return body + " sig=" + me.Firmar(body)
}

func entrante(door, entry string) string {
	d, m := verificarPuerta(door), rxEntrada.FindStringSubmatch(strings.TrimSpace(entry))
	if d == nil || m == nil || m[2] != idPuerta(door) {
		return ""
	}
	t, _ := strconv.ParseInt(m[4], 10, 64)
	n, _ := strconv.ParseInt(m[5], 10, 64)
	if t > d.exp || !peajeOk(door, m[1], n, d.zeros) {
		return ""
	}
	if !Verificar(m[1], fmt.Sprintf("C2 m=%s door=%s dev=%s t=%d n=%d", m[1], m[2], m[3], t, n), m[6]) {
		return ""
	}
	return m[1]
}

// DispositivoDe is the device fingerprint of a door entry.
func DispositivoDe(entry string) string {
	if m := rxEntrada.FindStringSubmatch(strings.TrimSpace(entry)); m != nil {
		return m[3]
	}
	return ""
}

func verificarDesde(chain []string, memberKey, first string) bool {
	if len(chain) == 0 {
		return false
	}
	by := first
	for i, c := range chain {
		m := rxCert.FindStringSubmatch(c)
		if m == nil || m[2] != by || !Verificar(m[2], "C1 m="+m[1]+" by="+m[2], m[3]) {
			return false
		}
		by = m[1]
		if i == len(chain)-1 && m[1] != memberKey {
			return false
		}
	}
	return true
}

// VerificarCadena checks a membership chain back to one of founders (Certificates.verifyChain).
func VerificarCadena(chain []string, memberKey string, founders ...string) bool {
	es := func(k string) bool {
		for _, f := range founders {
			if f == k {
				return true
			}
		}
		return false
	}
	if len(chain) >= 2 && strings.HasPrefix(chain[0], "OD1 ") {
		d, e := verificarPuerta(chain[0]), entrante(chain[0], chain[1])
		if d == nil || e == "" || !es(d.by) {
			return false
		}
		if len(chain) == 2 {
			return e == memberKey
		}
		return verificarDesde(chain[2:], memberKey, e)
	}
	if len(chain) == 0 {
		return false
	}
	m := rxCert.FindStringSubmatch(chain[0])
	return m != nil && es(m[2]) && verificarDesde(chain, memberKey, m[2])
}

/* ---------- the member-list entry (net/directory.dart) ---------- */

type Entrada struct {
	Key, Zone, Reflector string
	Accuracy             int64
	T                    int64
	Sig                  string
}

func (e Entrada) cuerpo() string {
	return fmt.Sprintf("D3|%s|%s|%s|%d|%d", e.Key, e.Zone, e.Reflector, e.Accuracy, e.T)
}

// NuevaEntrada signs an entry without a place (Zonda: the cell is 0,0, its zone "0,0").
func NuevaEntrada(me *Identidad, t int64) Entrada {
	e := Entrada{Key: me.Publica(), Zone: "0,0", T: t}
	e.Sig = me.Firmar(e.cuerpo())
	return e
}

func (e Entrada) Verificada() bool { return Verificar(e.Key, e.cuerpo(), e.Sig) }

func (e Entrada) JSON() Msg {
	m := Msg{"k": e.Key, "z": e.Zone, "a": e.Accuracy, "t": e.T, "sig": e.Sig}
	if e.Reflector != "" {
		m["refl"] = e.Reflector
	}
	return m
}

func EntradaDe(j Msg) (Entrada, bool) {
	k, ok1 := Texto(j, "k")
	z, ok2 := Texto(j, "z")
	t, ok3 := Entero(j, "t")
	s, ok4 := Texto(j, "sig")
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return Entrada{}, false
	}
	r, _ := Texto(j, "refl")
	a, _ := Entero(j, "a")
	return Entrada{k, z, r, a, t, s}, true
}

/* ---------- presence card (social/social.dart) ---------- */

type Tarjeta struct {
	Key, Name, Gender, Bio, Photo, Zone string
	Age                                 *int64
	CellX, CellY                        *int64
	Likes, Photos                       []string
	Dating                              bool
	At                                  int64
	Sig                                 string
}

func opt(p *int64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatInt(*p, 10)
}

func (c Tarjeta) cuerpo() string {
	s := fmt.Sprintf("PC1|%s|%s|%s|%s|%s|%s|%s|%s|%d|%s", c.Key, c.Name, opt(c.Age), c.Gender, opt(c.CellX), opt(c.CellY), strings.Join(c.Likes, ","), c.Bio, c.At, c.Photo)
	if !(c.Zone == "" && len(c.Photos) < 2) {
		s += "|z=" + c.Zone + "|ps=" + strings.Join(c.Photos, ",")
	}
	if c.Dating {
		s += "|dt=1"
	}
	return s
}

// Contenido is everything the card says except its time: two cards with the same content are the same card.
func (c Tarjeta) Contenido() string {
	return strings.Replace(c.cuerpo(), fmt.Sprintf("|%d|", c.At), "||", 1)
}

func (c Tarjeta) Verificada() bool { return Verificar(c.Key, c.cuerpo(), c.Sig) }

// NuevaTarjeta signs this member's card (no age, no place: Zonda asks only nick, gender and a photo).
func NuevaTarjeta(me *Identidad, name, gender, photo string, at int64) Tarjeta {
	c := Tarjeta{Key: me.Publica(), Name: name, Gender: gender, Photo: photo, At: at}
	if photo != "" {
		c.Photos = []string{photo}
	}
	c.Sig = me.Firmar(c.cuerpo())
	return c
}

func (c Tarjeta) JSON() Msg {
	m := Msg{"key": c.Key, "name": c.Name, "g": c.Gender, "likes": []any{}, "bio": c.Bio, "at": c.At, "sig": c.Sig}
	if c.Age != nil {
		m["age"] = *c.Age
	} else {
		m["age"] = nil
	}
	if c.CellX != nil && c.CellY != nil {
		m["cell"] = Msg{"x": *c.CellX, "y": *c.CellY}
	} else {
		m["cell"] = nil
	}
	if c.Photo != "" {
		m["ph"] = c.Photo
	} else {
		m["ph"] = nil
	}
	if c.Zone != "" {
		m["z"] = c.Zone
	}
	if len(c.Photos) > 0 {
		m["ps"] = c.Photos
	}
	if c.Dating {
		m["dt"] = 1
	}
	return m
}

func TarjetaDe(j Msg) (Tarjeta, bool) {
	k, ok1 := Texto(j, "key")
	n, ok2 := Texto(j, "name")
	at, ok3 := Entero(j, "at")
	s, ok4 := Texto(j, "sig")
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return Tarjeta{}, false
	}
	c := Tarjeta{Key: k, Name: n, At: at, Sig: s, Gender: "m"}
	if g, ok := Texto(j, "g"); ok {
		c.Gender = g
	}
	if a, ok := Entero(j, "age"); ok {
		c.Age = &a
	}
	if cell, ok := Objeto(j, "cell"); ok {
		x, okx := Entero(cell, "x")
		y, oky := Entero(cell, "y")
		if okx && oky {
			c.CellX, c.CellY = &x, &y
		}
	}
	c.Likes = Textos(j["likes"])
	c.Bio, _ = Texto(j, "bio")
	c.Photo, _ = Texto(j, "ph")
	if z, ok := j["z"]; ok && z != nil {
		c.Zone = Interpolar(z)
	}
	c.Photos = Textos(j["ps"])
	if d, ok := Entero(j, "dt"); ok && d == 1 {
		c.Dating = true
	}
	return c, true
}

/* ---------- rooms with an owner (social/private_room.dart) ---------- */

func b64u(s string) string { return base64.URLEncoding.EncodeToString([]byte(s)) }
func unb64u(s string) (string, bool) {
	b, err := base64.URLEncoding.DecodeString(s)
	return string(b), err == nil
}

func campos(line, tag string) map[string]string {
	parts := strings.Split(strings.TrimSpace(line), " ")
	if len(parts) == 0 || parts[0] != tag {
		return nil
	}
	out := map[string]string{}
	for _, p := range parts[1:] {
		i := strings.Index(p, "=")
		if i <= 0 {
			return nil
		}
		out[p[:i]] = p[i+1:]
	}
	return out
}

func lineaFirmada(by *Identidad, body string) string { return body + " sig=" + by.Firmar(body) }

func lineaVerificada(line, signer string) bool {
	i := strings.LastIndex(line, " sig=")
	return i >= 0 && Verificar(signer, line[:i], line[i+5:])
}

func atoi(s string) (int64, bool) {
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil
}

type Carta struct {
	Line, ID, Name, Creator string
	T                       int64
}

func NuevaCarta(creator *Identidad, name string, t int64) Carta {
	id := Hash(fmt.Sprintf("RC1|%s|%s|%d", creator.Publica(), name, t))[:16]
	c, _ := LeerCarta(lineaFirmada(creator, fmt.Sprintf("RC1 id=%s name=%s creator=%s t=%d", id, b64u(name), creator.Publica(), t)))
	return c
}

func LeerCarta(line string) (Carta, bool) {
	f := campos(line, "RC1")
	if f == nil {
		return Carta{}, false
	}
	name, ok := unb64u(f["name"])
	t, okt := atoi(f["t"])
	if !ok || !okt || f["id"] == "" || f["creator"] == "" || !lineaVerificada(line, f["creator"]) {
		return Carta{}, false
	}
	if f["id"] != Hash(fmt.Sprintf("RC1|%s|%s|%d", f["creator"], name, t))[:16] {
		return Carta{}, false
	}
	return Carta{strings.TrimSpace(line), f["id"], name, f["creator"], t}, true
}

type Admision struct {
	Line, Room, Member, By string
	Epoch, T               int64
}

func NuevaAdmision(by *Identidad, room, member string, epoch, t int64) Admision {
	a, _ := LeerAdmision(lineaFirmada(by, fmt.Sprintf("RA1 room=%s member=%s by=%s epoch=%d t=%d", room, member, by.Publica(), epoch, t)))
	return a
}

func LeerAdmision(line string) (Admision, bool) {
	f := campos(line, "RA1")
	e, ok1 := atoi(f["epoch"])
	t, ok2 := atoi(f["t"])
	if f == nil || !ok1 || !ok2 || f["room"] == "" || f["member"] == "" || f["by"] == "" || !lineaVerificada(line, f["by"]) {
		return Admision{}, false
	}
	return Admision{strings.TrimSpace(line), f["room"], f["member"], f["by"], e, t}, true
}

type Rotacion struct {
	Line, Room, By string
	Epoch, T       int64
	Roster, Banned map[string]bool
}

func conjunto(s string) map[string]bool {
	out := map[string]bool{}
	for _, k := range strings.Split(s, ",") {
		if k != "" {
			out[k] = true
		}
	}
	return out
}

func listaOrdenada(keys map[string]bool) string {
	l := make([]string, 0, len(keys))
	for k := range keys {
		l = append(l, k)
	}
	ordenar(l)
	return strings.Join(l, ",")
}

func NuevaRotacion(creator *Identidad, room string, epoch int64, roster, banned map[string]bool, t int64) Rotacion {
	r, _ := LeerRotacion(lineaFirmada(creator, fmt.Sprintf("RK1 room=%s by=%s epoch=%d roster=%s banned=%s t=%d", room, creator.Publica(), epoch, listaOrdenada(roster), listaOrdenada(banned), t)))
	return r
}

func LeerRotacion(line string) (Rotacion, bool) {
	f := campos(line, "RK1")
	e, ok1 := atoi(f["epoch"])
	t, ok2 := atoi(f["t"])
	_, okr := f["roster"]
	_, okb := f["banned"]
	if f == nil || !ok1 || !ok2 || !okr || !okb || f["room"] == "" || f["by"] == "" || !lineaVerificada(line, f["by"]) {
		return Rotacion{}, false
	}
	return Rotacion{strings.TrimSpace(line), f["room"], f["by"], e, t, conjunto(f["roster"]), conjunto(f["banned"])}, true
}

type Cierre struct {
	Line, Room, By string
	T              int64
}

func NuevoCierre(creator *Identidad, room string, t int64) Cierre {
	c, _ := LeerCierre(lineaFirmada(creator, fmt.Sprintf("RX1 room=%s by=%s t=%d", room, creator.Publica(), t)))
	return c
}

func LeerCierre(line string) (Cierre, bool) {
	f := campos(line, "RX1")
	t, ok := atoi(f["t"])
	if f == nil || !ok || f["room"] == "" || f["by"] == "" || !lineaVerificada(line, f["by"]) {
		return Cierre{}, false
	}
	return Cierre{strings.TrimSpace(line), f["room"], f["by"], t}, true
}

type Canales struct {
	Line, Room, By string
	Channels       []string
	T              int64
}

func NuevosCanales(creator *Identidad, room string, channels []string, t int64) Canales {
	l := make([]any, len(channels))
	for i, c := range channels {
		l[i] = c
	}
	c, _ := LeerCanales(lineaFirmada(creator, fmt.Sprintf("RV1 room=%s by=%s channels=%s t=%d", room, creator.Publica(), b64u(Codificar(l)), t)))
	return c
}

func LeerCanales(line string) (Canales, bool) {
	f := campos(line, "RV1")
	t, ok := atoi(f["t"])
	raw, okr := unb64u(f["channels"])
	if f == nil || !ok || !okr || f["room"] == "" || f["by"] == "" || !lineaVerificada(line, f["by"]) {
		return Canales{}, false
	}
	v, okv := DecodificarValor([]byte(raw))
	if !okv {
		return Canales{}, false
	}
	var list []string
	for _, c := range Textos(v) {
		if c = strings.TrimSpace(c); c != "" {
			list = append(list, c)
		}
	}
	if len(list) == 0 {
		return Canales{}, false
	}
	return Canales{strings.TrimSpace(line), f["room"], f["by"], list, t}, true
}

/* ---------- the room's secret key of one epoch (RoomKey) ---------- */

func NuevaLlaveSala() []byte {
	k := make([]byte, 32)
	rand.Read(k)
	return k
}

// SellarSala: nonce + ciphertext under the room key (pinenacl SecretBox).
func SellarSala(key, plain []byte) []byte {
	var k [32]byte
	copy(k[:], key)
	var n [24]byte
	rand.Read(n[:])
	return secretbox.Seal(n[:], plain, &n, &k)
}

func AbrirSala(key, sealed []byte) ([]byte, bool) {
	if len(sealed) < 24+secretbox.Overhead {
		return nil, false
	}
	var k [32]byte
	copy(k[:], key)
	var n [24]byte
	copy(n[:], sealed[:24])
	return secretbox.Open(nil, sealed[24:], &n, &k)
}
