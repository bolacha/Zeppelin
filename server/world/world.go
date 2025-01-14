package world

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"github.com/zeppelinmc/zeppelin/protocol/properties"
	"github.com/zeppelinmc/zeppelin/server/session"
	"github.com/zeppelinmc/zeppelin/server/world/chunk"
	"github.com/zeppelinmc/zeppelin/server/world/dimension"
	"github.com/zeppelinmc/zeppelin/server/world/level"
	"github.com/zeppelinmc/zeppelin/server/world/terrain"
	"github.com/zeppelinmc/zeppelin/util/log"
)

type World struct {
	level.Level
	dimensions map[string]*dimension.Dimension
	Broadcast  *session.Broadcast

	levelPrepared bool

	props properties.ServerProperties

	lock *os.File

	path              string
	worldAge, dayTime atomic.Int64
}

const version = 19133

func NewWorld(props properties.ServerProperties) (*World, error) {
	var err error
	w := &World{
		path:      props.LevelName,
		Broadcast: session.NewBroadcast(props),
		props:     props,
	}

	owgen := terrain.NewTerrainGenerator(int64(w.Data.WorldGenSettings.Seed))

	w.Level, err = level.Open(props.LevelName)
	if err != nil {
		w.prepareLevel(owgen, props)
	}

	if w.Level.Data.VersionInt > version {
		return nil, fmt.Errorf("world is too old!")
	}
	if w.Level.Data.VersionInt < version {
		return nil, fmt.Errorf("world is too new!")
	}

	if w.obtainLock() != nil {
		return nil, fmt.Errorf("failed to obtain session.lock")
	}

	w.worldAge.Store(w.Level.Data.Time)
	w.dayTime.Store(w.Level.Data.DayTime)
	w.dimensions = map[string]*dimension.Dimension{
		"minecraft:overworld": dimension.New(
			props.LevelName+"/region",
			"minecraft:overworld",
			"minecraft:overworld",
			w.Broadcast,
			owgen,
			w.Level,
		),
	}

	w.Level.Refresh(w.props)

	return w, nil
}

// prepareLevel creates a new level.dat file and other world folders
func (w *World) prepareLevel(owgen chunk.Generator, props properties.ServerProperties) {
	w.Level = level.New(owgen, props, w.path)

	os.MkdirAll(w.path+"/playerdata", 0755)

	os.Mkdir(w.path+"/region", 0755)
	os.Mkdir(w.path+"/poi", 0755)
	os.Mkdir(w.path+"/entities", 0755)

	os.MkdirAll(w.path+"/DIM-1/region", 0755)
	os.MkdirAll(w.path+"/DIM1/region", 0755)
}

func (w *World) obtainLock() error {
	f, err := os.OpenFile(w.path+"/session.lock", os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	f.Write(level.SessionLock)
	w.lock = f
	return nil
}

// returns the dimension struct for the dimension name
func (w *World) Dimension(name string) *dimension.Dimension {
	if !strings.Contains(name, ":") {
		name = "minecraft:" + name
	}

	return w.dimensions[name]
}

func (w *World) Save() {
	for _, dim := range w.dimensions {
		dim.Save()
	}
	w.lock.Close()
	log.Infoln("Closed world")
}

func (w *World) RegisterDimension(name string, dim *dimension.Dimension) {
	w.dimensions[name] = dim
}

// increments the day time and world age by one tick and returns the updated time
func (w *World) IncrementTime() (worldAge, dayTime int64) {
	worldAge = w.worldAge.Add(1)
	dayTime = w.dayTime.Add(1)

	return
}

func (w *World) Time() (worldAge, dayTime int64) {
	return w.worldAge.Load(), w.dayTime.Load()
}

func (w *World) DaytimeAdd(delta int64) {
	w.dayTime.Add(delta)
}

func (w *World) DaytimeSet(v int64) {
	w.dayTime.Store(v)
}

func (w *World) WorldAgeSet(v int64) {
	w.worldAge.Store(v)
}

func (w *World) LoadedChunks() int32 {
	var count int32

	for _, dim := range w.dimensions {
		count += dim.LoadedChunks()
	}

	return count
}
