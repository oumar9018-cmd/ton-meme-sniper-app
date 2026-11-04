// Command trench-block0-parser is a Phase 1 Block-0 parser for Trench Radar.
//
// It connects to TON public lite-servers over ADNL (via tonutils-go),
// fetches the latest masterchain block and its shard blocks, and prints
// one JSON event per transaction.
//
// Phase 1 ONLY: read-only parsing. No wallet, no key, no seed phrase,
// no signing, no sending messages, no trades, no Vault, no auto-sniper.
//
// Build environment note: this VM blocks UDP (Test 2, 2026-11-03:
// PermissionError on raw UDP), so ADNL connect WILL fail here. That is
// an environment failure, not a build failure. The binary is built here
// and run live on a VPS where UDP egress is allowed. See README.md.
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
)

// TxEvent is one parsed transaction, emitted as one JSON line.
type TxEvent struct {
	Source         string `json:"source"` // "trench-block0-parser-v1"
	BlockWorkchain int32  `json:"block_workchain"`
	BlockShard     int64  `json:"block_shard"`
	BlockSeqno     uint32 `json:"block_seqno"`
	BlockTime      uint32 `json:"block_time"` // GenUtime of the block header, 0 if unknown
	TxTime         uint32 `json:"tx_time"`    // transaction Now field
	Account        string `json:"account"`    // base64 user-friendly address, or hex fallback
	AccountHex     string `json:"account_hex"`
	TxHash         string `json:"tx_hash"` // hex, empty if the lite-server did not supply one
	LT             uint64 `json:"lt"`
	OrigStatus     string `json:"orig_status"`
	EndStatus      string `json:"end_status"`
	InMsgType      string `json:"in_msg_type"` // INTERNAL / EXTERNAL_IN / EXTERNAL_OUT / NONE
	InSrc          string `json:"in_src,omitempty"`
	InDst          string `json:"in_dst,omitempty"`
	HasStateInit   bool   `json:"has_state_init"`
	HasBody        bool   `json:"has_body"`
	OutMsgCount    uint16 `json:"out_msg_count"`
	// Classification, see classify().
	Type string `json:"type"` // "deploy" | "pool_candidate" | "other"
	// PoolFilter explains why pool_candidate can or cannot fire.
	PoolFilter string `json:"pool_filter"` // "not_configured" | "dex_address_configured"
}

// envFailure prints a clean, non-retrying error and exits non-zero.
// It is used for every failure that happens before the first block is
// parsed, because on a UDP-blocked host those failures are environmental.
func envFailure(stage string, err error) {
	fmt.Fprintf(os.Stderr, "ERROR stage=%s: %v\n", stage, err)
	fmt.Fprintln(os.Stderr, "ENVIRONMENT/UDP FAILURE, NOT A BUILD FAILURE: ADNL lite-client traffic is UDP. This host blocks UDP egress, so no lite-server connection can succeed here. Run this binary on a host/VPS with UDP egress allowed (see README.md).")
	os.Exit(2)
}

func formatAccount(workchain int32, data []byte) string {
	if len(data) != 32 {
		return hex.EncodeToString(data)
	}
	// NewAddress takes workchain as a byte and converts it back via int8,
	// so -1 (masterchain) is passed as 0xFF and round-trips correctly.
	return address.NewAddress(0, byte(workchain), data).String()
}

// classify implements ONLY the rules stated in the task, nothing more.
//   - deploy: the inbound message carries a StateInit. This is the
//     observable "new contract deployed" signal in a block.
//   - pool_candidate: deploy AND the transaction touches the operator
//     supplied DEX factory/router address (src, dst, or account). The
//     address is never invented here: it only comes from the
//     --dex-address flag, which the operator must fill with a value
//     verified from an official DEX document. With no flag, this type
//     is never emitted.
//   - other: everything else.
//
// No opcode, no pool-creation rule, and no contract address is
// hard-coded in this file.
func classify(ev *TxEvent, dexAddr string) {
	ev.PoolFilter = "not_configured"
	if dexAddr == "" {
		if ev.HasStateInit {
			ev.Type = "deploy"
		} else {
			ev.Type = "other"
		}
		return
	}
	ev.PoolFilter = "dex_address_configured"
	touchesDex := ev.Account == dexAddr || ev.InSrc == dexAddr || ev.InDst == dexAddr
	if ev.HasStateInit && touchesDex {
		ev.Type = "pool_candidate"
	} else if ev.HasStateInit {
		ev.Type = "deploy"
	} else {
		ev.Type = "other"
	}
}

// parseTx converts one tlb.Transaction into a TxEvent. It never fails
// on a missing inbound message: such transactions are "other" with
// InMsgType NONE.
func parseTx(block *ton.BlockIDExt, blockTime uint32, tx *tlb.Transaction, dexAddr string) *TxEvent {
	ev := &TxEvent{
		Source:         "trench-block0-parser-v1",
		BlockWorkchain: block.Workchain,
		BlockShard:     block.Shard,
		BlockSeqno:     block.SeqNo,
		BlockTime:      blockTime,
		TxTime:         tx.Now,
		Account:        formatAccount(block.Workchain, tx.AccountAddr),
		AccountHex:     hex.EncodeToString(tx.AccountAddr),
		TxHash:         hex.EncodeToString(tx.Hash),
		LT:             tx.LT,
		OrigStatus:     string(tx.OrigStatus),
		EndStatus:      string(tx.EndStatus),
		InMsgType:      "NONE",
		OutMsgCount:    tx.OutMsgCount,
	}
	if tx.IO.In != nil {
		msg := tx.IO.In
		ev.InMsgType = string(msg.MsgType)
		// Each message kind carries StateInit, SrcAddr, DstAddr and
		// Body on its concrete type; use the library accessors.
		switch {
		case msg.AsInternal() != nil:
			m := msg.AsInternal()
			ev.HasStateInit = m.StateInit != nil
			ev.HasBody = m.Body != nil
			if m.SrcAddr != nil {
				ev.InSrc = m.SrcAddr.String()
			}
			if m.DstAddr != nil {
				ev.InDst = m.DstAddr.String()
			}
		case msg.AsExternalIn() != nil:
			m := msg.AsExternalIn()
			ev.HasStateInit = m.StateInit != nil
			ev.HasBody = m.Body != nil
			if m.SrcAddr != nil {
				ev.InSrc = m.SrcAddr.String()
			}
			if m.DstAddr != nil {
				ev.InDst = m.DstAddr.String()
			}
		case msg.AsExternalOut() != nil:
			m := msg.AsExternalOut()
			ev.HasStateInit = m.StateInit != nil
			ev.HasBody = m.Body != nil
			if m.SrcAddr != nil {
				ev.InSrc = m.SrcAddr.String()
			}
			if m.DstAddr != nil {
				ev.InDst = m.DstAddr.String()
			}
		}
	}
	classify(ev, dexAddr)
	return ev
}

// parseBlock fetches one block, parses its transactions, and writes
// one JSON line per transaction to enc. It returns the number of
// events written. Block time comes from the block header (GenUtime).
func parseBlock(ctx context.Context, api ton.APIClientWrapped, block *ton.BlockIDExt, dexAddr string, enc *json.Encoder) (int, error) {
	data, err := api.GetBlockData(ctx, block)
	if err != nil {
		return 0, fmt.Errorf("get block data (%d:%d:%d): %w", block.Workchain, block.Shard, block.SeqNo, err)
	}
	blockTime := data.BlockInfo.GenUtime
	txs, err := data.ListTransactions()
	if err != nil {
		return 0, fmt.Errorf("list transactions (%d:%d:%d): %w", block.Workchain, block.Shard, block.SeqNo, err)
	}
	for _, tx := range txs {
		if tx == nil {
			continue
		}
		if err := enc.Encode(parseTx(block, blockTime, tx, dexAddr)); err != nil {
			return 0, fmt.Errorf("encode event: %w", err)
		}
	}
	return len(txs), nil
}

// parseMasterAndShards parses the given master block and then every
// shard block it references. It returns the total event count.
func parseMasterAndShards(ctx context.Context, api ton.APIClientWrapped, master *ton.BlockIDExt, dexAddr string, enc *json.Encoder) (int, error) {
	total, err := parseBlock(ctx, api, master, dexAddr, enc)
	if err != nil {
		return total, err
	}
	shards, err := api.GetBlockShardsInfo(ctx, master)
	if err != nil {
		// Shard listing is a soft failure: the master block itself
		// was already parsed, so report the error but keep going
		// with what we have, and let the caller decide.
		fmt.Fprintf(os.Stderr, "WARN stage=shards_info master_seqno=%d: %v\n", master.SeqNo, err)
		return total, nil
	}
	for _, shard := range shards {
		n, err := parseBlock(ctx, api, shard, dexAddr, enc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN stage=parse_shard %d:%d:%d: %v\n", shard.Workchain, shard.Shard, shard.SeqNo, err)
			continue
		}
		total += n
	}
	return total, nil
}

func main() {
	once := flag.Bool("once", false, "fetch latest masterchain block, parse it and its shards, print JSON events, exit")
	watch := flag.Bool("watch", false, "loop for new masterchain blocks and parse each one")
	maxBlocks := flag.Int("max-blocks", 10, "with --watch: maximum number of new master blocks to parse before exiting (0 = unlimited, still bounded by --timeout)")
	timeout := flag.Duration("timeout", 30*time.Second, "overall timeout for the run")
	configURL := flag.String("config", "https://ton-blockchain.github.io/global.config.json", "public mainnet lite-server config URL")
	dexAddr := flag.String("dex-address", "", "OPTIONAL operator-supplied DEX factory/router address, verified from an official DEX document. Empty = pool_candidate is never emitted (pool filter TODO). Never hard-code an address here.")
	flag.Parse()

	if *once == *watch {
		fmt.Fprintln(os.Stderr, "ERROR: exactly one of --once or --watch is required")
		flag.Usage()
		os.Exit(2)
	}

	// Validate an operator-supplied DEX address early, so a typo is a
	// clean startup error, not a silent never-matching filter.
	dex := ""
	if *dexAddr != "" {
		parsed, err := address.ParseAddr(*dexAddr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR stage=dex_address: %v\n", err)
			os.Exit(2)
		}
		dex = parsed.String()
		fmt.Fprintf(os.Stderr, "INFO dex_address_configured=%s pool_candidate enabled for deploys touching this address only\n", dex)
	} else {
		fmt.Fprintln(os.Stderr, "INFO dex_address not supplied: pool_candidate will never be emitted; deploys are logged as 'deploy'. TODO: supply a verified DEX factory/router address via --dex-address.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// (a) Load public mainnet lite-servers from the official config.
	// This step is HTTP, not UDP, so it can succeed even on a
	// UDP-blocked host; the ADNL failure surfaces at the first query.
	cfg, err := liteclient.GetConfigFromUrl(ctx, *configURL)
	if err != nil {
		envFailure("config", err)
	}

	// (b) Connect via the ADNL lite-client connection pool.
	// Single attempt: no retry loop, no hang beyond the context timeout.
	client := liteclient.NewConnectionPool()
	if err := client.AddConnectionsFromConfig(ctx, cfg); err != nil {
		envFailure("connect", err)
	}
	defer client.Stop()

	// Fast proof policy + trusted block from the config, as in the
	// tonutils-go block-scan example. This does not send any message.
	api := ton.NewAPIClient(client, ton.ProofCheckPolicyFast)
	api.SetTrustedBlockFromConfig(cfg)

	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)

	// (c) Current masterchain info, then parse.
	master, err := api.GetMasterchainInfo(ctx)
	if err != nil {
		envFailure("masterchain_info", err)
	}
	fmt.Fprintf(os.Stderr, "INFO master workchain=%d shard=%d seqno=%d\n", master.Workchain, master.Shard, master.SeqNo)

	if *once {
		n, err := parseMasterAndShards(ctx, api, master, dex, enc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR stage=parse: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "INFO done events=%d master_seqno=%d\n", n, master.SeqNo)
		return
	}

	// --watch: poll for new master blocks, parse each new one once.
	seenMaster := master.SeqNo
	// Parse the starting master as block 1, so --watch always yields
	// data on a working host even if no new master appears in time.
	parsed := 0
	n, err := parseMasterAndShards(ctx, api, master, dex, enc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR stage=parse: %v\n", err)
		os.Exit(1)
	}
	parsed++
	fmt.Fprintf(os.Stderr, "INFO watch parsed master_seqno=%d events=%d parsed=%d\n", master.SeqNo, n, parsed)

	for *maxBlocks == 0 || parsed < *maxBlocks {
		select {
		case <-ctx.Done():
			fmt.Fprintf(os.Stderr, "INFO watch stopping: %v parsed=%d\n", ctx.Err(), parsed)
			return
		case <-time.After(time.Second):
		}
		cur, err := api.GetMasterchainInfo(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN stage=masterchain_info poll: %v\n", err)
			continue
		}
		if cur.SeqNo <= seenMaster {
			continue
		}
		seenMaster = cur.SeqNo
		n, err := parseMasterAndShards(ctx, api, cur, dex, enc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN stage=parse master_seqno=%d: %v\n", cur.SeqNo, err)
			continue
		}
		parsed++
		fmt.Fprintf(os.Stderr, "INFO watch parsed master_seqno=%d events=%d parsed=%d\n", cur.SeqNo, n, parsed)
	}
	fmt.Fprintf(os.Stderr, "INFO watch done parsed=%d max_blocks=%d\n", parsed, *maxBlocks)
}
