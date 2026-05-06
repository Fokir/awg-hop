package amnezia

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

	"awghop/internal/domain"
)

// AmneziaWG требует ненулевых, попарно уникальных и не совпадающих с типами
// сообщений WireGuard (1, 2, 3, 4) маркеров H1..H4 — иначе `awg setconf`
// возвращает `Invalid argument`. Аналогично S1/S2 (junk-padding для
// handshake init/response) — должны быть заданы и не приводить к коллизии
// размеров пакетов рукопожатия. Для AmneziaWG 2.0+ нужны и S3/S4 (padding для
// cookie reply и transport): без них в экспортированном клиентском конфиге
// клиент и сервер расходятся по формату пакетов после handshake — трафик не
// ходит (см. amnezia-vpn/amnezia-client#2219). Этот файл — единственный
// источник истины для генерации дефолтов.

const (
	headerMarkerMin uint32 = 5
	headerMarkerMax uint32 = 1<<31 - 1

	junkPaddingMin = 15
	junkPaddingMax = 150

	// initiationOverhead — размер пакета handshake initiation в WireGuard.
	// При S1+initiationOverhead == S2 размер padded-init совпал бы с
	// размером handshake response — AmneziaWG это запрещает.
	initiationOverhead = 56
)

// EnsureAmneziaDefaults проставляет криптостойкие случайные значения для
// S1/S2/S3/S4/H1..H4, если они отсутствуют или нулевые. Возвращает true,
// если хоть одно поле было изменено — вызывающий код может на этом
// основании сохранить настройки в БД.
//
// H1..H4 трактуются как «не заданы», если все четыре равны нулю; это
// единственное надёжное разделение «свежей записи» и «осознанно выставленных
// маркеров», т.к. правомерных нулевых маркеров AmneziaWG не допускает.
func EnsureAmneziaDefaults(in *domain.IngressSettings) (bool, error) {
	changed := false

	if strings.TrimSpace(in.S1) == "" {
		v, err := randomJunkPadding()
		if err != nil {
			return false, err
		}
		in.S1 = strconv.Itoa(v)
		changed = true
	}
	if strings.TrimSpace(in.S2) == "" {
		s1, _ := strconv.Atoi(strings.TrimSpace(in.S1))
		v, err := randomJunkPaddingExcluding(s1, s1+initiationOverhead)
		if err != nil {
			return false, err
		}
		in.S2 = strconv.Itoa(v)
		changed = true
	}

	s1v, _ := strconv.Atoi(strings.TrimSpace(in.S1))
	s2v, _ := strconv.Atoi(strings.TrimSpace(in.S2))
	if strings.TrimSpace(in.S3) == "" {
		v, err := randomJunkPaddingExcluding(s1v, s2v)
		if err != nil {
			return false, err
		}
		in.S3 = strconv.Itoa(v)
		changed = true
	}
	s3v, _ := strconv.Atoi(strings.TrimSpace(in.S3))
	if strings.TrimSpace(in.S4) == "" {
		v, err := randomJunkPaddingExcluding(s1v, s2v, s3v)
		if err != nil {
			return false, err
		}
		in.S4 = strconv.Itoa(v)
		changed = true
	}

	if in.H1 == 0 && in.H2 == 0 && in.H3 == 0 && in.H4 == 0 {
		hs, err := randomHeaderMarkers()
		if err != nil {
			return false, err
		}
		in.H1 = int64(hs[0])
		in.H2 = int64(hs[1])
		in.H3 = int64(hs[2])
		in.H4 = int64(hs[3])
		changed = true
	}

	return changed, nil
}

func randomUint32(min, max uint32) (uint32, error) {
	if max <= min {
		return 0, fmt.Errorf("invalid range: [%d, %d]", min, max)
	}
	span := uint64(max - min + 1)
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0, err
	}
	v := binary.BigEndian.Uint64(buf[:]) % span
	return min + uint32(v), nil
}

func randomHeaderMarkers() ([4]uint32, error) {
	// Запрещены: 0, стандартные WG типы 1..4, и повторы.
	taken := map[uint32]struct{}{0: {}, 1: {}, 2: {}, 3: {}, 4: {}}
	var out [4]uint32
	for i := 0; i < 4; i++ {
		for attempt := 0; attempt < 64; attempt++ {
			v, err := randomUint32(headerMarkerMin, headerMarkerMax)
			if err != nil {
				return out, err
			}
			if _, dup := taken[v]; dup {
				continue
			}
			taken[v] = struct{}{}
			out[i] = v
			break
		}
		if out[i] == 0 {
			return out, fmt.Errorf("failed to generate unique header marker for H%d", i+1)
		}
	}
	return out, nil
}

func randomJunkPadding() (int, error) {
	v, err := randomUint32(uint32(junkPaddingMin), uint32(junkPaddingMax))
	if err != nil {
		return 0, err
	}
	return int(v), nil
}

func randomJunkPaddingExcluding(forbidden ...int) (int, error) {
	bad := make(map[int]struct{}, len(forbidden))
	for _, v := range forbidden {
		bad[v] = struct{}{}
	}
	for attempt := 0; attempt < 128; attempt++ {
		v, err := randomJunkPadding()
		if err != nil {
			return 0, err
		}
		if _, dup := bad[v]; dup {
			continue
		}
		return v, nil
	}
	return 0, fmt.Errorf("failed to generate junk padding excluding %v", forbidden)
}
