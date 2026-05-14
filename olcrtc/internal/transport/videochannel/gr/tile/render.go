package tile

func (c *Codec) renderFrame(wire []byte) []byte {
	frame := make([]byte, FrameW*FrameH)
	for i := range frame {
		frame[i] = White
	}
	c.renderSync(frame, 0, c.syncRows)
	c.renderSync(frame, c.rows-c.syncRows, c.rows)

	m := c.cfg.Module
	maxMod := c.cols * c.dataRows

	for byteIdx := 0; byteIdx < len(wire) && byteIdx*8 < maxMod; byteIdx++ {
		b := wire[byteIdx]
		if b == 0 {
			continue
		}
		for bit := 0; bit < 8; bit++ {
			if (b>>(7-bit))&1 == 0 {
				continue
			}
			modIdx := byteIdx*8 + bit
			if modIdx >= maxMod {
				break
			}
			col := modIdx % c.cols
			row := c.syncRows + modIdx/c.cols
			x0, y0 := col*m, row*m
			for dy := 0; dy < m; dy++ {
				base := (y0+dy)*FrameW + x0
				for dx := 0; dx < m; dx++ {
					frame[base+dx] = Black
				}
			}
		}
	}
	return frame
}

func (c *Codec) readFrame(frame []byte) []byte {
	m := c.cfg.Module
	half := m / 2

	totalMods := c.cols * c.dataRows
	out := make([]byte, (totalMods+7)/8)
	for modIdx := 0; modIdx < totalMods; modIdx++ {
		col := modIdx % c.cols
		row := c.syncRows + modIdx/c.cols
		cx := col*m + half
		cy := row*m + half
		if cx >= FrameW {
			cx = FrameW - 1
		}
		if cy >= FrameH {
			cy = FrameH - 1
		}
		if frame[cy*FrameW+cx] < 128 {
			out[modIdx/8] |= 1 << (7 - modIdx%8)
		}
	}
	return out
}

func (c *Codec) renderSync(frame []byte, rowStart, rowEnd int) {
	m := c.cfg.Module
	for row := rowStart; row < rowEnd; row++ {
		for col := 0; col < c.cols; col++ {
			color := White
			if col%2 == 0 {
				color = Black
			}
			x0, y0 := col*m, row*m
			for dy := 0; dy < m; dy++ {
				base := (y0+dy)*FrameW + x0
				for dx := 0; dx < m; dx++ {
					frame[base+dx] = color
				}
			}
		}
	}
}
