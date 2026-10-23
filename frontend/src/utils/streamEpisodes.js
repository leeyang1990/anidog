// Match the backend's explicit-number-first policy, including descending
// provider playlists. Positions are usable only for entirely unnumbered roads.
export function streamEpisodePayload(episodes) {
  const numbered = episodes.map(ep => {
    const label = String(ep.name || '').trim().match(/(?:^|[^0-9])(?:第\s*|EP(?:ISODE)?\s*)?(\d{1,4})(?:\.\d+)?(?:\s*[集话話])?(?:$|[^0-9])/i)
    const path = String(ep.url || '').match(/[-_/](\d{1,4})(?:\.html?)?(?:[?#].*)?$/)
    const number = Number((label || path)?.[1])
    return { url: ep.url, name: ep.name, number: number > 0 ? number : 0, road_name: ep.road_name || '' }
  })
  if (numbered.some(ep => ep.number > 0)) return numbered.filter(ep => ep.number > 0)
  return numbered.map((ep, index) => ({ ...ep, number: index + 1 }))
}
