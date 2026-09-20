package drearchive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/api/manifest"
)

type Snapshot struct {
	Manifest     manifest.Manifest
	Events       []ioevent.IOEvent
	VectorGraph  manifest.VectorGraph
	RedactionLog manifest.RedactionLog
}

func Encrypt(plain []byte, key string) ([]byte, error) {
	k := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(k[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	sealed := gcm.Seal(nil, nonce, plain, nil)
	return append(nonce, sealed...), nil
}

func Decrypt(data []byte, key string) ([]byte, error) {
	k := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(k[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func PackTarGz(m manifest.Manifest, events []byte, graph manifest.VectorGraph, redact manifest.RedactionLog) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	writeFile := func(name string, data []byte) error {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err := tw.Write(data)
		return err
	}

	mb, _ := json.MarshalIndent(m, "", "  ")
	if err := writeFile("manifest.json", mb); err != nil {
		return nil, err
	}
	if err := writeFile("events.bin", events); err != nil {
		return nil, err
	}
	gb, _ := json.MarshalIndent(graph, "", "  ")
	if err := writeFile("vector_graph.json", gb); err != nil {
		return nil, err
	}
	rb, _ := json.MarshalIndent(redact, "", "  ")
	if err := writeFile("redaction_log.json", rb); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func ParseTarGz(data []byte) (*Snapshot, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	var snap Snapshot
	files := map[string][]byte{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		buf := make([]byte, hdr.Size)
		if _, err := io.ReadFull(tr, buf); err != nil {
			return nil, err
		}
		files[hdr.Name] = buf
	}

	if err := json.Unmarshal(files["manifest.json"], &snap.Manifest); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(files["vector_graph.json"], &snap.VectorGraph); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(files["redaction_log.json"], &snap.RedactionLog); err != nil {
		return nil, err
	}
	eventsReader := bytes.NewReader(files["events.bin"])
	for {
		evt, err := ioevent.Decode(eventsReader)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		snap.Events = append(snap.Events, evt)
	}
	return &snap, nil
}

func OpenFile(path, key string) (*Snapshot, error) {
	enc, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	plain, err := Decrypt(enc, key)
	if err != nil {
		return nil, err
	}
	return ParseTarGz(plain)
}
