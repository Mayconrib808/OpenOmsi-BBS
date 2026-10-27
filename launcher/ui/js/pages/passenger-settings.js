// The same canonical map key as the game's local passenger effects.
export function mapKey(value) {
  return String(value || '').replaceAll('\\', '/').toLowerCase().replace(/\/+$/, '')
    .replace(/\/global\.cfg$/, '').split('/').pop().normalize('NFD').replace(/[\u0300-\u036f]/g, '').replace(/[^a-z0-9]/g, '');
}
export function enabledMaps(value) {
  return new Set(String(value || '').split(';').filter((x) => x.trim()).map(mapKey));
}
export function withMap(value, map, on) {
  const key = mapKey(map);
  const maps = String(value || '').split(';').filter((x) => x.trim() && mapKey(x) !== key);
  if (on && key) maps.push(key);
  return maps.join(';');
}
