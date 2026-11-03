package nucleo

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

/* ---------- private chats (DirectMessage, DM2) ---------- */

// EnviarMensaje seals text (and an optional photo) for to and sends it: straight by a direct link, else as a sealed
// signal over the board, and over the mesh too.
func (n *Nodo) EnviarMensaje(to, texto string, foto []byte) {
	if n.Yo == nil || (strings.TrimSpace(texto) == "" && foto == nil) {
		return
	}
	at := ahora()
	id := "m" + strconv.FormatInt(at, 36) + fmt.Sprint(int(aleatorio(1)[0])%999)
	inner := Msg{"t": strings.TrimSpace(texto)}
	ph := ""
	if foto != nil {
		ph = n.guardarFoto(foto)
		inner["ph"] = ph
	}
	sealed, err := n.Yo.Sellar(to, []byte(Codificar(inner)))
	if err != nil {
		return
	}
	box := hex.EncodeToString(sealed)
	from := n.Clave()
	dm := Msg{"id": id, "from": from, "to": to, "at": at, "box": box, "sig": n.Yo.Firmar(fmt.Sprintf("DM2|%s|%s|%s|%d|%s", id, from, to, at, box))}
	n.Chats[to] = append(n.Chats[to], LineaChat{Texto: strings.TrimSpace(texto), Mia: true, At: at, Foto: ph, ID: id, Estado: "sending"})
	n.Contactos[to] = true
	env := Msg{"t": "dm", "dm": dm, "chain": n.Cadena}
	liviano := Copiar(env)
	if foto != nil {
		if b, err := n.Yo.SellarGrande(to, foto); err == nil {
			env["b64"] = base64.StdEncoding.EncodeToString(b)
		}
	}
	if !n.Router.InundarHacia(VecinoNet(to), env, 1) {
		n.senalPor(to, Msg{"kind": "dm", "m": liviano}, false)
		if foto != nil {
			n.Red.Querer(to)
		}
		n.Router.Inundar(env, 6)
	}
	n.guardarChats()
	n.cambio()
}

func (n *Nodo) alDm(m Msg) {
	j, ok := Objeto(m, "dm")
	if !ok || n.Yo == nil {
		return
	}
	id, _ := Texto(j, "id")
	from, _ := Texto(j, "from")
	to, _ := Texto(j, "to")
	at, okat := Entero(j, "at")
	box, _ := Texto(j, "box")
	sig, _ := Texto(j, "sig")
	if !okat || to != n.Clave() || !Verificar(from, fmt.Sprintf("DM2|%s|%s|%s|%d|%s", id, from, to, at, box), sig) || !n.EsMiembro(from, m["chain"]) {
		return
	}
	raw, err := hex.DecodeString(box)
	if err != nil {
		return
	}
	plain, ok := n.Yo.Abrir(from, raw)
	if !ok {
		return
	}
	inner, ok := Decodificar(string(plain))
	if !ok {
		return
	}
	ph, _ := Texto(inner, "ph")
	if b64, ok := Texto(m, "b64"); ok && ph != "" {
		if sealed, err := base64.StdEncoding.DecodeString(b64); err == nil {
			if b, ok := n.Yo.AbrirGrande(from, sealed); ok && HashBytes(b) == ph {
				n.Alm.GuardarFoto(ph, b)
			}
		}
	}
	if ph == "" || n.Alm.Foto(ph) != nil {
		n.EnviarSenal(from, Msg{"kind": "dm-ok", "id": id})
	} else {
		n.PedirFoto(ph, from)
	}
	for _, l := range n.Chats[from] {
		if l.ID == id {
			return
		}
	}
	n.Chats[from] = append(n.Chats[from], LineaChat{Texto: Interpolar(inner["t"]), At: at, Foto: ph, ID: id})
	n.Contactos[from] = true
	n.guardarChats()
	n.cambio()
}

func (n *Nodo) confirmado(from, id string) {
	for i, l := range n.Chats[from] {
		if l.ID == id && l.Mia {
			n.Chats[from][i].Estado = "ok"
			n.guardarChats()
			n.cambio()
		}
	}
}

// AgregarContacto keeps key among the contacts (their beacons are followed, so they are found again).
func (n *Nodo) AgregarContacto(key string) {
	n.Contactos[key] = true
	n.guardarChats()
	n.Tablero.seguir()
	n.cambio()
}

func (n *Nodo) guardarChats() {
	chats := Msg{}
	for k, ls := range n.Chats {
		var out []any
		for _, l := range ls {
			out = append(out, Msg{"t": l.Texto, "me": l.Mia, "at": l.At, "ph": l.Foto, "id": l.ID, "st": l.Estado})
		}
		chats[k] = out
	}
	n.Alm.EscribirJSON("chats", chats)
	var cs []string
	for k := range n.Contactos {
		cs = append(cs, k)
	}
	n.Alm.EscribirJSON("contactos", cs)
	names := Msg{}
	for k := range n.Contactos {
		names[k] = n.NombreDe(k)
	}
	n.Alm.EscribirJSON("nombres", names)
}

func (n *Nodo) cargarChats() {
	if v, ok := n.Alm.LeerJSON("chats"); ok {
		for k, ls := range v.(map[string]any) {
			for _, x := range ls.([]any) {
				o := x.(map[string]any)
				at, _ := Entero(o, "at")
				n.Chats[k] = append(n.Chats[k], LineaChat{Texto: Interpolar(o["t"]), Mia: o["me"] == true, At: at, Foto: Interpolar(o["ph"]), ID: Interpolar(o["id"]), Estado: Interpolar(o["st"])})
			}
		}
	}
	if v, ok := n.Alm.LeerJSON("contactos"); ok {
		for _, k := range Textos(v) {
			n.Contactos[k] = true
		}
	}
	if v, ok := n.Alm.LeerJSON("nombres"); ok {
		for k, nm := range v.(map[string]any) {
			n.nombres[k] = Interpolar(nm)
		}
	}
}
