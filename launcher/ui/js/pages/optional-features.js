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
  { type: 'toggle', key: 'ai_driver_wave', label: 'AI bus drivers wave when greeting', help: host },
  { type: 'toggle', key: 'passenger_ambient_talk', label: 'Optional passenger conversations inside buses', help: restart },
  { type: 'toggle', key: 'ai_bus_overtaking', label: 'AI buses safely pass other stopped buses', help: host },
  { type: 'toggle', key: 'cars_yield_to_buses', label: 'Cars change lanes for buses leaving stops', help: host },
  { type: 'toggle', key: 'illegal_parking', label: 'Temporary illegally parked AI cars', help: host },
  { type: 'toggle', key: 'independent_passenger_demand', label: 'Independent passenger demand at each stop', help: host },
];

export const optionalRows = (traffic) => OPTIONAL_FEATURE_ROWS.filter((row) =>
  (['extra_traffic_density', 'ai_bus_greetings', 'ai_driver_wave', 'ai_bus_overtaking', 'cars_yield_to_buses', 'illegal_parking'].includes(row.key)) === traffic);
