package persistence

import (
	"bufio"
	"encoding/gob"
	"encoding/json"
	"io"
	"log"
	"os"
	"raft/service"
	"strings"
)

type GobPersistence struct {
	stateFile *os.File
	logFile   *os.File

	logEncoder   *gob.Encoder
	stateEncoder *gob.Encoder

	offsets []int64
}

func saveFile(stateDir string, agentId string, fname string, flags int) *os.File {
	logPathParts := []string{stateDir, strings.Replace(agentId, ":", "_", -1), fname}

	perm := 0x180
	fileExec := 0x40

	logDir := strings.Join(logPathParts[:2], string(os.PathSeparator))
	logPath := strings.Join(logPathParts, string(os.PathSeparator))

	os.MkdirAll(logDir, os.FileMode(perm|fileExec))

	file, err := os.OpenFile(logPath, flags, os.FileMode(perm))

	if err != nil {
		log.Fatal("Failed to open file: ", fname, err)
	}

	return file
}

func NewGobPersistence(configDir string, agentId string) *GobPersistence {
	stateFile := saveFile(configDir, agentId, "state", os.O_CREATE|os.O_RDWR)
	logFile := saveFile(configDir, agentId, "log", os.O_CREATE|os.O_RDWR)
	return &GobPersistence{
		stateFile:    stateFile,
		logFile:      logFile,
		offsets:      make([]int64, 0),
		stateEncoder: gob.NewEncoder(stateFile),
		logEncoder:   gob.NewEncoder(logFile),
	}
}

func (p *GobPersistence) Close() {
	p.stateFile.Close()
	p.logFile.Close()
}

func (p *GobPersistence) SaveState(savedState SavedState) {
	p.stateFile.Truncate(0)
	p.stateFile.Seek(0, os.SEEK_SET)

	bytes, err := json.Marshal(savedState)

	if err != nil {
		log.Fatal("failed to marshal json", err)
	}

	_, err = p.stateFile.Write(bytes)

	if err != nil {
		log.Fatal("Failed writing new state")
	}
	err = p.stateFile.Sync()

	if err != nil {
		log.Fatal("Failed to persist")
	}
}

func (p *GobPersistence) ReadState() SavedState {
	p.stateFile.Seek(0, os.SEEK_SET)

	bytes := make([]byte, 1024)

	n, err := p.stateFile.Read(bytes)

	var result SavedState

	if err == io.EOF {
		return result
	}

	if err != nil {
		log.Fatal("Failed to read file", n, err)
	}

	if n == 1024 {
		log.Fatal("State too large")
	}

	err = json.Unmarshal(bytes[:n], &result)

	if err != nil {
		log.Fatal("Failed to read state", err, bytes[:n])
	}
	return result
}

func (p *GobPersistence) ReadLogs() []service.Entry {
	p.logFile.Seek(0, os.SEEK_SET)
	scanner := bufio.NewScanner(p.logFile)

	result := make([]service.Entry, 0)

	for scanner.Scan() {
		entry := new(service.Entry)

		pos, err := p.logFile.Seek(0, os.SEEK_CUR)
		p.offsets = append(p.offsets, pos)

		err = json.Unmarshal(scanner.Bytes(), entry)
		if err != nil {
			log.Fatal("Error unmarshalling log", err)
		}
		result = append(result, *entry)
	}

	return result
}

func (p *GobPersistence) AddLogs(entries ...service.Entry) {
	for _, entry := range entries {
		bytes, err := json.Marshal(entry)

		if err != nil {
			log.Fatal("Failed to marshal logs")
		}

		pos, err := p.logFile.Seek(0, os.SEEK_CUR)

		if err != nil {
			log.Fatal("Could not get file position")
		}

		p.offsets = append(p.offsets, pos)

		_, err = p.logFile.Write(bytes)

		if err != nil {
			log.Fatal("Failed to write new log contentes", err)
		}

		if _, err := p.logFile.WriteString("\n"); err != nil {
			log.Fatal("Error appending new line", err)
		}

		if err != nil {
			log.Fatal("Failed to write log", err)
		}

	}
	p.logFile.Sync()
}

func (p *GobPersistence) Truncate(index int64) {
	p.logFile.Truncate(p.offsets[index-1])
	p.logFile.Seek(p.offsets[index-1], 0)
	p.logFile.Sync()
}
