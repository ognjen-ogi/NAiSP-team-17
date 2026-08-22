package blockmanager

import (
	"fmt"
	"io"
	"os"

	"github.com/ognjen-ogi/NAiSP-team-17/internal/storage/blockcache"
)

//Jedini sloj u sistemu koji sme direktno da cita/pise fajlove na disku

type BlockManager struct {
	blockSize int //velicina jednog bloka u B(4096,8192 ili 16384)
	cache     *blockcache.BlockCache
}

// Pravi novi BlockManager sa zadatom velicinom bloka
func NewBlockManager(blockSize int, cache *blockcache.BlockCache) *BlockManager {
	return &BlockManager{blockSize: blockSize, cache: cache}
}

// ReadBlock cita tacno jedan blok(blockSize bajtova) iz fajla na datoj putanji
// blockNumber je redni broj bloka(0,1,2,...)
func (bm *BlockManager) ReadBlock(path string, blockNumber int64) ([]byte, error) {
	//1. Prvo pitamo kes
	if data, found := bm.cache.Get(path, blockNumber); found {
		return data, nil
	}
	//kes-miss,moramo na disk
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("Ne mogu da otvorim fajl %s:%w", path, err)
	}
	// defer znači "izvrši ovo kad funkcija završi, bez obzira kako se završila"
	// (uspešno ili sa greškom). Ovo je Go idiom da se fajl uvek zatvori.
	defer file.Close()

	//racunamo tacnu poziciju(offset) gde pocinje trazeni blok
	offset := blockNumber * int64(bm.blockSize)

	// Seek pomera "pokazivač čitanja" na tu poziciju u fajlu.
	// io.SeekStart znači "offset se računa od početka fajla" (a ne od kraja ili trenutne pozicije).
	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("ne mogu da pozicioniram na offset %d: %w", offset, err)
	}
	//pravimo prazan buffer tacno velicine bloka i punimo ga
	buffer := make([]byte, bm.blockSize)
	// io.ReadFull čita TAČNO len(buffer) bajtova, ili vraća grešku ako ih nema dovoljno.
	// Ovo je bitno - obično 'file.Read()' ume da pročita manje nego što tražiš,
	// a nama treba garancija da smo dobili ceo blok
	_, err = io.ReadFull(file, buffer)
	if err != nil {
		return nil, fmt.Errorf("Ne mogu da procitam blok %d:%w", blockNumber, err)
	}

	//upisujemo u kes za naredni put PRE nego sto vratimo rezultat
	return buffer, nil

}

// WriteBlock upisuje dati sadržaj kao blok na datu poziciju u fajlu.
// Ako fajl ne postoji, kreira se. Ako je sadržaj manji od blockSize, dopunjava se nulama (padding)

func (bm *BlockManager) WriteBlock(path string, blockNumber int64, data []byte) error {
	//Sadrzaj ne sme biti veci od jednog bloka
	if len(data) > bm.blockSize {
		return fmt.Errorf("Podaci(%d bajtova) su veci od velicine bloka(%d bajtova)", len(data), bm.blockSize)
	}
	// O_RDWR: otvori za čitanje i pisanje
	// O_CREATE: napravi fajl ako ne postoji
	// 0644: standardna Unix dozvola za fajl (vlasnik čita/piše, ostali samo čitaju)
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("ne mogu da otvorim/napravim fajl %s: %w", path, err)
	}
	defer file.Close()

	offset := blockNumber * int64(bm.blockSize)
	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		return fmt.Errorf("ne mogu da pozicioniram na offset %d: %w", offset, err)
	}
	// PADDING: ako su podaci manji od blockSize, moramo da upišemo CEO blok
	// (spec eksplicitno traži padding gde je potrebno). Napravimo bafer pun nula
	// veličine celog bloka i prekopiramo naše podatke na početak
	paddedBlock := make([]byte, bm.blockSize)
	copy(paddedBlock, data)

	_, err = file.Write(paddedBlock)
	if err != nil {
		return fmt.Errorf("Ne mogu da upišem blok %d: %w", blockNumber, err)
	}
	//Azuriramo kes sa paddovanim sadrzajem-identican onome na disku(kad bi neko citao blok, da se poklapa sa diskom)
	bm.cache.Put(path, blockNumber, paddedBlock)

	return nil
}
