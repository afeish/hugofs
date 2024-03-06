package consensus

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Jille/raft-grpc-leader-rpc/leaderhealth"
	transport "github.com/Jille/raft-grpc-transport"
	"github.com/Jille/raftadmin"
	boltdb "github.com/hashicorp/raft-boltdb"
	"github.com/pingcap/log"
	"go.uber.org/zap"

	"github.com/hashicorp/raft"
	"google.golang.org/grpc"
)

func Run(ctx context.Context, dir, id, addr, peers string, isBootstrap bool, server *grpc.Server) *Scheduler {
	r, tm, err := newRaft(ctx, dir, id, addr, peers, nil, isBootstrap)
	if err != nil {
		log.Error("failed to start raft", zap.Error(err))
	}
	scheduler := newScheduler(r, func(sig chan struct{}) {
		t := time.NewTicker(time.Second * 5)
		defer t.Stop()

		for {
			select {
			case <-t.C:
				// fmt.Println("schedule job called")
			case <-sig:
				return
			}
		}
	})

	tm.Register(server)
	leaderhealth.Setup(r, server, []string{id})
	raftadmin.Register(server, r)
	return scheduler
}

func newRaft(ctx context.Context, rootDir, myID, myAddress, _peers string, fsm raft.FSM, raftBootstrap bool) (r *raft.Raft, tm *transport.Manager, err error) {
	c := raft.DefaultConfig()
	c.LocalID = raft.ServerID(myID)
	c.LogLevel = "off"

	baseDir := filepath.Join(rootDir, myID)
	os.MkdirAll(baseDir, 0755)

	ldb, err := boltdb.NewBoltStore(filepath.Join(baseDir, "logs.dat"))
	if err != nil {
		return nil, nil, fmt.Errorf(`boltdb.NewBoltStore(%q): %v`, filepath.Join(baseDir, "logs.dat"), err)
	}

	sdb, err := boltdb.NewBoltStore(filepath.Join(baseDir, "stable.dat"))
	if err != nil {
		return nil, nil, fmt.Errorf(`boltdb.NewBoltStore(%q): %v`, filepath.Join(baseDir, "stable.dat"), err)
	}

	fss, err := raft.NewFileSnapshotStore(baseDir, 3, os.Stderr)
	if err != nil {
		return nil, nil, fmt.Errorf(`raft.NewFileSnapshotStore(%q, ...): %v`, baseDir, err)
	}

	tm = transport.New(raft.ServerAddress(myAddress), []grpc.DialOption{grpc.WithInsecure()}) //lint:ignore SA1019 ignore

	r, err = raft.NewRaft(c, fsm, ldb, sdb, fss, tm.Transport())
	if err != nil {
		return nil, nil, fmt.Errorf("raft.NewRaft: %v", err)
	}

	log.Info("raft started", zap.Any("peers", _peers))

	var servers []raft.Server = []raft.Server{
		{
			Suffrage: raft.Voter,
			ID:       raft.ServerID(myID),
			Address:  raft.ServerAddress(myAddress),
		},
	}

	peers, _ := parsePeers(_peers)
	for k, v := range peers {
		if v == myAddress || k == myID {
			continue
		}
		servers = append(servers, raft.Server{
			Suffrage: raft.Voter,
			ID:       raft.ServerID(k),
			Address:  raft.ServerAddress(v),
		})
	}

	if raftBootstrap {
		cfg := raft.Configuration{
			Servers: servers,
		}

		f := r.BootstrapCluster(cfg)
		if err := f.Error(); err != nil {
			return r, tm, fmt.Errorf("raft.Raft.BootstrapCluster: %v", err)
		}
	}

	return r, tm, nil
}

func parsePeers(_peers string) (map[string]string, error) {
	if _peers == "" {
		return nil, nil
	}

	hostList := strings.Split(_peers, ",")
	peers := make(map[string]string)
	for _, host := range hostList {
		parts := strings.Split(host, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid peer host: %q", host)
		}
		peers[parts[0]] = host
	}
	return peers, nil
}
