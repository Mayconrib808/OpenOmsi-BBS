// The saved switches take effect on the next game start, in both settings interfaces.
const restart = 'Takes effect when the game starts the next time';
const host = 'In multiplayer, shared AI vehicles are controlled by the host. Restart the game after changing this option.';
export const OPTIONAL_FEATURE_ROWS = [
  { type: 'toggle', key: 'bcs_event_scenes', label: 'BCS event scenes', help: host },
  { type: 'toggle', key: 'bcs_inspectors', label: 'BCS ticket inspectors', help: restart },
  { type: 'toggle', key: 'passenger_door_comments', label: 'Passenger door and missed-stop comments', help: restart },
  { type: 'toggle', key: 'passenger_exit_thanks', label: 'Passengers thank the driver when alighting', help: restart },
  { type: 'toggle', key: 'extra_traffic_density', label: 'Extra traffic on wide roads', help: host },
  { type: 'toggle', key: 'ai_bus_greetings', label: 'AI bus greetings', help: host },
  { type: 'toggle', key: 'passenger_connections_enabled', label: 'Passenger connections between lines', help: restart },
];

export const optionalRows = (traffic) => OPTIONAL_FEATURE_ROWS.filter((row) =>
  (row.key === 'extra_traffic_density' || row.key === 'ai_bus_greetings') === traffic);
