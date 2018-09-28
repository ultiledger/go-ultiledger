package ledger

// CloseInfoBuffer caches unclosed ledger close info until
// local ledger state catch up with the network state.
type CloseInfoBuffer struct {
	infos []*CloseInfo
}

// Append info the the tail of the buffer by checking whether
// the new info is the expected next-to-the-sequence info.
func (b *CloseInfoBuffer) Append(info *CloseInfo) {
	if info == nil {
		return
	}
	if len(b.infos) > 0 {
		lastInfo := b.infos[len(b.infos)-1]
		if lastInfo.Index+1 != info.Index {
			return
		}
	}
	b.infos = append(b.infos, info)
}

// Return the first CloseInfo without removing it
func (b *CloseInfoBuffer) PeekHead() *CloseInfo {
	if len(b.infos) == 0 {
		return nil
	}
	return b.infos[0]
}

// Return the first CloseInfo and remove it from internal buffer
func (b *CloseInfoBuffer) PopHead() *CloseInfo {
	if len(b.infos) == 0 {
		return nil
	}
	h := b.infos[0]
	b.infos = b.infos[1:]
	return h
}
