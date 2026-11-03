// Writes interop fixtures from the Dart core (the reference), so the Go port proves it speaks the same bytes:
//   dart --packages=E:/Zonda/app/.dart_tool/package_config.json herramientas/fixturas.dart > nucleo/testdata/fixturas.json
import 'dart:convert';
import 'dart:typed_data';

import 'package:zonda_core/zonda_core.dart';

void main() {
  final a = Identity.fromSeed(Uint8List.fromList(List.generate(32, (i) => i + 1)));
  final b = Identity.fromSeed(Uint8List.fromList(List.generate(32, (i) => 200 - i)));
  final big = Uint8List.fromList(List.generate(Identity.piece + 1000, (i) => i % 251));
  final roomKey = Uint8List.fromList(List.generate(32, (i) => i * 3 % 256));
  final nk = NostrKeys.of(a);
  final ev = NostrEvent.create(nk, kind: 30078, tags: [['d', 'zonda'], ['t', 'zd-k-abc']], content: jsonEncode({'x': 'ñandú "q" \n\t<tag> & é \u0001'}), createdAt: 1800000000);
  final entry = DirEntry.create(a, cell: const Cell(0, 0), t: 1800000000000);
  final charter = RoomCharter.create(a, name: 'Los Pibes ñ', t: 1800000000001);
  final card = PresenceCard.create(a, name: 'Ana', gender: 'f', photo: 'ab' * 32, at: 1800000000002);
  final door = OfficialNetwork.openDoor;
  final nonce = OpenDoor.work(door, b.publicKey)!;
  final entryLine = OpenDoor.enter(b, door, nonce, 1800000000003, device: 'dispositivo');
  final dm = DirectMessage.create(a, to: b.publicKey, id: 'm1', text: 'hola', at: 1800000000004);
  print(const JsonEncoder.withIndent(' ').convert({
    'a_secret': a.exportSecret(),
    'a_pub': a.publicKey,
    'b_secret': b.exportSecret(),
    'b_pub': b.publicKey,
    'a_sign_hola': a.sign('hola'),
    'seal_a_to_b': toHex(a.seal(b.publicKey, Uint8List.fromList(utf8.encode('secreto ñ')))),
    'seal_large_a_to_b_b64': base64Encode(a.sealLarge(b.publicKey, big)),
    'hash_x': Identity.hash('x'),
    'photo_hash_big': SocialNode.photoHash(big),
    'room_key_b64': base64Encode(roomKey),
    'room_sealed_b64': base64Encode(RoomKey.seal(roomKey, Uint8List.fromList(utf8.encode('en la sala')))),
    'nostr_priv': nk.priv,
    'nostr_pub': nk.pub,
    'nostr_event': ev,
    'dir_entry': entry.toJson(),
    'charter': charter.line,
    'charter_id': charter.id,
    'card': card.toJson(),
    'card_content': card.content,
    'door_entry': entryLine,
    'dm': dm.toJson(),
    'nick_tag_ana': Rendezvous.nickTag(' Ana '),
    'key_tag_a': Rendezvous.keyTag(a.publicKey),
  }));
}
