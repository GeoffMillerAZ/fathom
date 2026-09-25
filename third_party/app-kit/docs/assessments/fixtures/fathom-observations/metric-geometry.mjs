// Pure browser-side metric typography oracle. Pass this function directly to
// Playwright locator.evaluate/page.evaluate; it has no module or Node runtime dependencies.
export function inspectMetricTypography(main) {
  const rect = value => ({ left: value.left, right: value.right, top: value.top, bottom: value.bottom, width: value.width, height: value.height });
  const visibleRects = element => {
    const range = document.createRange();
    range.selectNodeContents(element);
    return [...range.getClientRects()].filter(value => value.width > 0 && value.height > 0).map(rect);
  };
  const contains = (outer, inner) => inner.left >= outer.left - 1 && inner.right <= outer.right + 1 && inner.top >= outer.top - 1 && inner.bottom <= outer.bottom + 1;
  const intersects = (a, b) => Math.min(a.right, b.right) - Math.max(a.left, b.left) > 0.5 && Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top) > 0.5;
  const values = [...main.querySelectorAll('.text-2xl')];
  const metrics = values.map((element, index) => {
    const pair = element.parentElement;
    const bandCell = element.closest('dd')?.parentElement;
    const tileCell = element.parentElement?.parentElement?.parentElement;
    const tileLabel = tileCell?.tagName === 'DIV' ? tileCell.querySelector(':scope > p') : null;
    const cell = bandCell || (tileLabel ? tileCell : null);
    const label = bandCell?.querySelector(':scope > dt') || tileLabel;
    const captions = pair ? [...pair.children].filter(child => child !== element && child.textContent.trim()) : [];
    const valueRects = visibleRects(element);
    const captionRects = captions.flatMap(visibleRects);
    const cellRect = cell ? rect(cell.getBoundingClientRect()) : null;
    const style = getComputedStyle(element);
    return {
      index,
      label: label?.textContent.trim() || null,
      value: element.textContent.trim(),
      caption: captions.map(item => item.textContent.trim()).join(' | '),
      valueRects,
      captionRects,
      cellRect,
      visible: element.checkVisibility(),
      fontSize: parseFloat(style.fontSize),
      fontWeight: Number(style.fontWeight),
      singleLine: valueRects.length === 1,
      bounded: Boolean(cellRect) && [...valueRects, ...captionRects].every(item => contains(cellRect, item)),
      separated: valueRects.every(valueRect => captionRects.every(captionRect => !intersects(valueRect, captionRect))),
    };
  });

  const candidates = [];
  for (const value of values) {
    const bandCell = value.closest('dd')?.parentElement;
    const tileCell = value.parentElement?.parentElement?.parentElement;
    const tileLabel = tileCell?.tagName === 'DIV' ? tileCell.querySelector(':scope > p') : null;
    const cell = bandCell || (tileLabel ? tileCell : null);
    if (!cell) { candidates.push({ element: value, role: 'value', label: null }); continue; }
    const label = bandCell?.querySelector(':scope > dt') || tileLabel;
    if (label) candidates.push({ element: label, role: 'label', label: label.textContent.trim() });
    candidates.push({ element: value, role: 'value', label: label?.textContent.trim() || null });
    for (const caption of value.parentElement?.children || []) {
      if (caption !== value && caption.textContent.trim()) candidates.push({ element: caption, role: 'caption', label: label?.textContent.trim() || null });
    }
  }
  const canvas = document.createElement('canvas');
  canvas.width = canvas.height = 1;
  const context = canvas.getContext('2d', { colorSpace: 'srgb', willReadFrequently: true });
  const rgba = color => {
    if (!CSS.supports('color', color)) return null;
    context.clearRect(0, 0, 1, 1);
    context.fillStyle = '#010203';
    context.fillStyle = color;
    context.fillRect(0, 0, 1, 1);
    return [...context.getImageData(0, 0, 1, 1).data];
  };
  const over = (front, back) => {
    const alpha = front[3] / 255, backAlpha = back[3] / 255;
    const outAlpha = alpha + backAlpha * (1 - alpha);
    if (outAlpha === 0) return [0, 0, 0, 0];
    return [0, 1, 2].map(index => Math.round((front[index] * alpha + back[index] * backAlpha * (1 - alpha)) / outAlpha)).concat(Math.round(outAlpha * 255));
  };
  const colors = candidates.map(({ element, role, label }, index) => {
    const lineage = [];
    for (let node = element; node; node = node.parentElement) lineage.unshift(node);
    let background = [0, 0, 0, 0];
    const unsupported = [];
    for (const node of lineage) {
      const style = getComputedStyle(node);
      if (parseFloat(style.opacity) !== 1) unsupported.push(`opacity:${style.opacity}`);
      if (style.backgroundImage !== 'none') unsupported.push(`background-image:${style.backgroundImage}`);
      if (style.mixBlendMode !== 'normal') unsupported.push(`mix-blend-mode:${style.mixBlendMode}`);
      if (style.filter && style.filter !== 'none') unsupported.push(`filter:${style.filter}`);
      if (style.backdropFilter && style.backdropFilter !== 'none') unsupported.push(`backdrop-filter:${style.backdropFilter}`);
      const layer = rgba(style.backgroundColor);
      if (!layer) unsupported.push(`background-color:${style.backgroundColor}`);
      else background = over(layer, background);
    }
    const style = getComputedStyle(element);
    const foreground = rgba(style.color);
    if (!foreground) unsupported.push(`color:${style.color}`);
    if (foreground && foreground[3] !== 255) unsupported.push(`foreground-alpha:${foreground[3]}`);
    if (background[3] !== 255) unsupported.push(`composited-background-alpha:${background[3]}`);
    return {
      index, role, label, text: element.textContent.trim(), color: style.color,
      foreground, background, unsupported,
      fontSize: parseFloat(style.fontSize), fontWeight: Number(style.fontWeight),
    };
  });
  return { metrics, colors };
}

// Pure browser-side shared-border oracle for stat bands. The trailing one-pixel
// grid offset must place last-row/last-column cell borders under a clipping
// wrapper while leaving measurable internal bottom/right separators.
export function inspectBandDividers(root) {
  const box = value => ({ left: value.left, right: value.right, top: value.top, bottom: value.bottom, width: value.width, height: value.height });
  const near = (a, b) => Math.abs(a - b) <= 0.6;
  const alpha = color => {
    if (!CSS.supports('color', color)) return null;
    const canvas = document.createElement('canvas'); canvas.width = canvas.height = 1;
    const context = canvas.getContext('2d', { colorSpace: 'srgb', willReadFrequently: true });
    context.clearRect(0, 0, 1, 1); context.fillStyle = '#010203'; context.fillStyle = color;
    context.fillRect(0, 0, 1, 1);
    return context.getImageData(0, 0, 1, 1).data[3];
  };
  const grids = [...root.querySelectorAll('dl')].filter(dl => {
    const cells = [...dl.children];
    return getComputedStyle(dl).display === 'grid' && cells.length > 0 && cells.every(cell => cell.querySelector(':scope > dt') && cell.querySelector(':scope > dd'));
  });
  return grids.map((grid, index) => {
    const wrapper = grid.parentElement;
    const wrapperStyle = getComputedStyle(wrapper);
    const wrapperRect = box(wrapper.getBoundingClientRect());
    const gridRect = box(grid.getBoundingClientRect());
    const wrapperBorder = { left: parseFloat(wrapperStyle.borderLeftWidth), top: parseFloat(wrapperStyle.borderTopWidth), right: parseFloat(wrapperStyle.borderRightWidth), bottom: parseFloat(wrapperStyle.borderBottomWidth) };
    const cellData = [...grid.children].map((cell, cellIndex) => {
      const style = getComputedStyle(cell);
      return {
        index: cellIndex, rect: box(cell.getBoundingClientRect()),
        rightWidth: parseFloat(style.borderRightWidth), bottomWidth: parseFloat(style.borderBottomWidth),
        rightStyle: style.borderRightStyle, bottomStyle: style.borderBottomStyle,
        rightAlpha: alpha(style.borderRightColor), bottomAlpha: alpha(style.borderBottomColor),
      };
    });
    const rowTops = [...new Set(cellData.map(cell => Math.round(cell.rect.top * 2) / 2))].sort((a, b) => a - b);
    const rows = rowTops.map(top => cellData.filter(cell => near(cell.rect.top, top)));
    const trailingRight = cellData.filter(cell => near(cell.rect.right, wrapperRect.right));
    const trailingBottom = cellData.filter(cell => near(cell.rect.bottom, wrapperRect.bottom));
    const innerLeft = wrapperRect.left + wrapperBorder.left, innerTop = wrapperRect.top + wrapperBorder.top;
    const innerRight = wrapperRect.right - wrapperBorder.right, innerBottom = wrapperRect.bottom - wrapperBorder.bottom;
    const vertical = cellData.filter(cell => !trailingRight.includes(cell)).map(cell => ({ left: cell.rect.right - cell.rightWidth, top: Math.max(cell.rect.top, innerTop), right: cell.rect.right, bottom: Math.min(cell.rect.bottom, innerBottom) }));
    const horizontal = cellData.filter(cell => !trailingBottom.includes(cell)).map(cell => ({ left: Math.max(cell.rect.left, innerLeft), top: cell.rect.bottom - cell.bottomWidth, right: Math.min(cell.rect.right, innerRight), bottom: cell.rect.bottom }));
    const cellsOwnBorders = cellData.every(cell => cell.rightWidth === 1 && cell.bottomWidth === 1 && cell.rightStyle !== 'none' && cell.bottomStyle !== 'none' && cell.rightAlpha > 0 && cell.bottomAlpha > 0);
    const clips = ['hidden', 'clip'].includes(wrapperStyle.overflowX) && ['hidden', 'clip'].includes(wrapperStyle.overflowY);
    const wrapperOwnsEdge = Object.values(wrapperBorder).every(width => width === 1) && parseFloat(wrapperStyle.borderTopLeftRadius) > 0;
    const offsetAtClipEdge = near(gridRect.right, wrapperRect.right) && near(gridRect.bottom, wrapperRect.bottom);
    const trailingRightClipped = trailingRight.length > 0 && trailingRight.every(cell => cell.rect.right - cell.rightWidth >= innerRight - 0.6 && cell.rect.right <= wrapperRect.right + 0.6);
    const trailingBottomClipped = trailingBottom.length > 0 && trailingBottom.every(cell => cell.rect.bottom - cell.bottomWidth >= innerBottom - 0.6 && cell.rect.bottom <= wrapperRect.bottom + 0.6);
    const internalBounds = [...vertical, ...horizontal].every(separator => separator.left >= innerLeft - 0.6 && separator.right <= innerRight + 0.6 && separator.top >= innerTop - 0.6 && separator.bottom <= innerBottom + 0.6);
    return {
      index, wrapper: wrapperRect, grid: gridRect, cells: cellData, rows: rows.length,
      vertical, horizontal, clips, wrapperOwnsEdge, offsetAtClipEdge,
      cellsOwnBorders, trailingRightClipped, trailingBottomClipped, internalBounds,
      valid: clips && wrapperOwnsEdge && offsetAtClipEdge && cellsOwnBorders && trailingRightClipped && trailingBottomClipped && internalBounds,
    };
  });
}
