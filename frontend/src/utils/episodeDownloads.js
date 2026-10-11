// A completed pack covers only episodes confirmed by the archive API. Its
// episode_number is the scheduling trigger, not its full coverage.
export function groupEpisodeDownloads(downloads, episodes) {
  const grouped = {}
  for (const ep of episodes) {
    grouped[ep.episode_number] = downloads.filter(d => {
      if (d.scope !== 'season') return d.episode_number === ep.episode_number
      if (d.status === 'completed') {
        return !d.media_missing && ep.status === 'completed' && ep.download_id === d.id
      }
      return d.status !== 'superseded' && d.episode_start > 0 &&
        ep.episode_number >= d.episode_start && ep.episode_number <= d.episode_end
    })
  }
  return grouped
}
