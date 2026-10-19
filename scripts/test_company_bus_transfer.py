#!/usr/bin/env python3
"""Independent encrypted client for an unchanged official server behind the Bridge.

Uses only synthetic bus/livery/texture files. Exercises native session proofs,
piece resume, quarantine, hashes and downloads through the public /tcp route.
"""
import base64
import hashlib
import json
import os
import socket
import struct
import sys
import unicodedata
from urllib.parse import urlparse
from urllib.request import urlopen

from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ed25519, x25519
from cryptography.hazmat.primitives.ciphers.aead import ChaCha20Poly1305
from cryptography.hazmat.primitives.kdf.hkdf import HKDF


class Stream:
    def __init__(self, url):
        u = urlparse(url)
        assert u.scheme == 'http' and u.hostname == '127.0.0.1'
        self.sock = socket.create_connection((u.hostname, u.port), timeout=12)
        self.file = self.sock.makefile('rb')
        key = base64.b64encode(os.urandom(16)).decode()
        self.sock.sendall((f'GET /tcp HTTP/1.1\r\nHost: {u.netloc}\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: {key}\r\n\r\n').encode())
        assert self.file.readline().startswith(b'HTTP/1.1 101')
        while self.file.readline() != b'\r\n':
            pass
        self.buf = bytearray()

    def send(self, data):
        # A native encrypted record is 65556 bytes: larger than a gameplay datagram.
        for offset in range(0, len(data), 100_000):
            body = data[offset:offset+100_000]
            n, mask = len(body), os.urandom(4)
            head = bytes([130, 128+n]) if n < 126 else (b'\x82\xfe'+struct.pack('!H', n) if n <= 65535 else b'\x82\xff'+struct.pack('!Q', n))
            self.sock.sendall(head+mask+bytes(b ^ mask[i % 4] for i, b in enumerate(body)))

    def exact(self, n):
        while len(self.buf) < n:
            head = self.file.read(2)
            assert len(head) == 2, 'WebSocket closed before all bytes arrived'
            op, size = head[0] & 15, head[1] & 127
            assert head[0] & 128 and not head[1] & 128
            if size == 126:
                size = struct.unpack('!H', self.file.read(2))[0]
            elif size == 127:
                size = struct.unpack('!Q', self.file.read(8))[0]
            assert size <= 128*1024
            body = self.file.read(size)
            assert len(body) == size and op == 2, ('Unexpected frame', op, body[:100])
            self.buf.extend(body)
        out = bytes(self.buf[:n]); del self.buf[:n]
        return out

    def line(self):
        out = bytearray()
        while not out.endswith(b'\n'):
            assert len(out) < 4096
            out.extend(self.exact(1))
        return bytes(out).strip()

    def close(self):
        self.sock.shutdown(socket.SHUT_RDWR)
        self.file.close(); self.sock.close()


class Secure:
    def __init__(self, url, session):
        self.raw = Stream(url)
        private = x25519.X25519PrivateKey.generate()
        public = private.public_key().public_bytes(serialization.Encoding.Raw, serialization.PublicFormat.Raw)
        self.raw.send(f'OMSIMODS/3 {session} {public.hex()}\n'.encode())
        words = self.raw.line().split()
        assert len(words) == 4 and words[0] == b'OK', words
        host, signer, signature = (bytes.fromhex(w.decode()) for w in words[1:])
        number = int(session, 16)
        assert int.from_bytes(hashlib.sha256(b'omsi2rw-host-key'+signer).digest()[:6], 'big') == number
        proof = f'OMSIMODS/3 {session} {public.hex()} {host.hex()}'.encode()
        ed25519.Ed25519PublicKey.from_public_bytes(signer).verify(signature, b'openOMSI host mods\x00'+proof)
        shared = private.exchange(x25519.X25519PublicKey.from_public_bytes(host))
        def key(direction):
            return HKDF(algorithm=hashes.SHA256(), length=32, salt=b'openOMSI LAN mods 2 '+number.to_bytes(8, 'big'), info=direction+public+host).derive(shared)
        self.tx, self.rx = ChaCha20Poly1305(key(b'client to host')), ChaCha20Poly1305(key(b'host to client'))
        self.sent = self.received = 0
        self.buf = bytearray()

    def send(self, data):
        for offset in range(0, len(data), 65536):
            cipher = self.tx.encrypt(b'\x00'*4+self.sent.to_bytes(8, 'big'), data[offset:offset+65536], b'')
            self.sent += 1
            self.raw.send(struct.pack('!I', len(cipher))+cipher)

    def exact(self, n):
        while len(self.buf) < n:
            size = struct.unpack('!I', self.raw.exact(4))[0]
            assert 16 <= size <= 65552
            self.buf.extend(self.rx.decrypt(b'\x00'*4+self.received.to_bytes(8, 'big'), self.raw.exact(size), b''))
            self.received += 1
        out = bytes(self.buf[:n]); del self.buf[:n]
        return out

    def line(self):
        out = bytearray()
        while not out.endswith(b'\n'):
            assert len(out) < 4096
            out.extend(self.exact(1))
        return bytes(out).strip()

    def ask(self, line, body=b''):
        self.send(line.encode()+b'\n'+body)
        return self.line()

    def sized(self, line, body=b''):
        reply = self.ask(line, body)
        assert reply.startswith(b'OK '), reply
        n = int(reply.split()[1]); assert 0 <= n <= 4*1024*1024
        return self.exact(n)

    def close(self):
        self.raw.close()


def fingerprint(manifest):
    def key(path):
        return unicodedata.normalize('NFC', path.lower()).encode()
    bus = key(manifest['bus'])
    data = b'omsi-vehicle-manifest/v2\x00sha256\x00'+struct.pack('<I', len(bus))+bus
    for e in sorted(manifest['entries'], key=lambda e: key(e['path'])):
        name = key(e['path'])
        data += struct.pack('<I', len(name))+name+struct.pack('<Q', e['size'])+bytes.fromhex(e['sha256'])
    return 'v2-'+hashlib.sha256(data).hexdigest()


def run(url, session):
    with urlopen(url+"/status", timeout=5) as response:
        assert json.load(response)["bbs_bus_transfers"] is True
    bad = Stream(url)
    bad.send(f'OMSIMODS/3 {int(session,16)^1:012x} {"ab"*32}\n'.encode())
    assert bad.line() == b'ERR not this session'
    bad.close()
    bus = f'Vehicles/Bridge-{os.urandom(8).hex()}/test.bus'
    folder = bus.rsplit('/', 1)[0]
    pixels = os.urandom(1024*512*3)
    bmp = b'BM'+struct.pack('<IHHI', 54+len(pixels), 0, 0, 54)+struct.pack('<IiiHHIIiiII', 40, 1024, 512, 1, 24, 0, len(pixels), 0, 0, 0, 0)+pixels
    files = {
        bus: b'[friendlyname]\nBridge synthetic '+os.urandom(8).hex().encode()+b'\nTest\nBus\n[mass]\n12000\n[boundingbox]\n2.5\n12\n3\n0\n0\n1.5\n',
        folder+'/Texture/test.bmp': bmp,
        folder+'/Texture/test.cti': b'[item]\nBridge livery\nrepaint_body\ntest.bmp\n',
    }
    entries = [dict(path=p, size=len(d), sha256=hashlib.sha256(d).hexdigest()) for p,d in sorted(files.items(), key=lambda e:e[0].lower())]
    manifest = dict(bus=bus, entries=entries)
    body = json.dumps(manifest).encode()
    c = Secure(url, session)
    listing = json.loads(c.sized('LIST'))
    assert listing['entries'] == [], 'Map sharing must remain disabled'
    unsafe = json.dumps(dict(bus='Vehicles/no/test.bus', entries=[dict(path='Vehicles/no/test.dll',size=2,sha256='ab'*32)])).encode()
    assert c.ask(f'OFFER {len(unsafe)}', unsafe).startswith(b'ERR ')
    plan = dict(json.loads(c.sized(f'OFFER2 {len(body)}', body)))
    bus_index = next(i for i,e in enumerate(entries) if e['path'] == bus)
    assert bus_index in plan
    wrong = bytes([files[bus][0]^1])+files[bus][1:]
    assert b'not the file the list names' in c.ask(f'PUT {bus_index} {len(wrong)}', wrong)
    plan = dict(json.loads(c.sized(f'OFFER2 {len(body)}', body)))
    texture_index = next(i for i,e in enumerate(entries) if e['path'].endswith('.bmp'))
    first = bmp[:1<<20]
    assert c.ask(f'PUTPIECE {texture_index} 0 {len(first)} - {len(first)}', first) == b'OK'
    c.close()
    c = Secure(url, session)
    plan = dict(json.loads(c.sized(f'OFFER2 {len(body)}', body)))
    assert plan[texture_index] == [1], 'First piece was not kept for resume'
    for i,pieces in plan.items():
        data = files[entries[i]['path']]
        for part in pieces:
            piece = data[part*(1<<20):(part+1)*(1<<20)]
            assert c.ask(f'PUTPIECE {i} {part} {len(piece)} - {len(piece)}', piece) == b'OK'
    assert c.ask('DONE').startswith(b'OK ')
    c.close()
    viewer = Secure(url, session)
    fp = fingerprint(manifest)
    actual = json.loads(viewer.sized('MANIFEST '+fp))
    assert fingerprint(actual) == fp and actual['bus'] == bus
    for e in actual['entries']:
        downloaded = viewer.sized('BLOB '+e['sha256'])
        assert downloaded == files[e['path']]
        assert hashlib.sha256(downloaded).hexdigest() == e['sha256']
    # Executable content disguised as bus data is refused by the native quarantine.
    evil = b'MZ'+b'X'*126
    refused = dict(bus=folder+'/bad.bus', entries=[dict(path=folder+'/bad.bus',size=len(evil),sha256=hashlib.sha256(evil).hexdigest())])
    offer = json.dumps(refused).encode()
    assert json.loads(viewer.sized(f'OFFER {len(offer)}', offer)) == [0]
    assert viewer.ask(f'PUT 0 {len(evil)}', evil).startswith(b'ERR ')
    viewer.close()
    print('PASS: native session proof; no map sharing; invalid hash and executable refused; 1 MiB piece resumed; exact bus, livery and texture downloaded by a second encrypted client.', flush=True)


if __name__ == '__main__':
    run(*sys.argv[1:])
