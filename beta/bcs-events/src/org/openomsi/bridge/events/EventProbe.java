package org.openomsi.bridge.events;

import java.io.InputStream;
import java.lang.instrument.Instrumentation;
import java.lang.management.ManagementFactory;
import java.lang.reflect.Field;
import java.nio.charset.StandardCharsets;
import java.nio.file.*;
import java.security.MessageDigest;
import java.time.Instant;
import java.util.*;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;

/** Passive beta: observes an already-running BCS event thread; never invokes game actions. */
public final class EventProbe {
    private static final AtomicBoolean STARTED = new AtomicBoolean();
    private static final String VERSION = "0.1.2";
    private static final String MANAGER = "system.q";
    private static final String POPUP = "o.j";
    private static final long HEARTBEAT_MS = 5000;
    private static final long POLL_MS = 250;

    public static void premain(String args, Instrumentation ignored) {
        try { start(args); }
        catch (Throwable unavailable) { System.err.println("[OpenOMSI events beta] Observer could not start"); }
    }
    public static void agentmain(String args, Instrumentation ignored) { premain(args, ignored); }

    private static void start(final String args) throws Exception {
        if (!STARTED.compareAndSet(false, true)) return;
        Path out = args == null || args.trim().isEmpty()
            ? Paths.get(System.getProperty("user.home"), ".openomsi", "bcs-events-beta")
            : Paths.get(args.trim());
        final Writer sink = new Writer(out);
        // Confirm loading synchronously, even for short JVMs such as java -version.
        sink.emit(status(sink, "agent_loaded", ""), "startup", System.currentTimeMillis());
        Thread worker = new Thread(new Runnable() {
            public void run() {
                try { observe(sink); }
                catch (Throwable unavailable) {
                    System.err.println("[OpenOMSI events beta] Capture unavailable: " + unavailable.getClass().getSimpleName());
                    try { sink.emit(status(sink, "observer_failed", unavailable.getClass().getSimpleName()), "status", System.currentTimeMillis()); }
                    catch (Throwable ignored) { }
                }
            }
        }, "OpenOMSI BCS event observer beta");
        worker.setDaemon(true);
        worker.setPriority(Thread.MIN_PRIORITY);
        worker.start();
    }

    private static Map<String,Object> status(Writer sink, String state, String detail) {
        Map<String,Object> row = new LinkedHashMap<String,Object>();
        row.put("schema", 1); row.put("probe_version", VERSION); row.put("pid", sink.pid);
        row.put("status", state); row.put("phase", "unavailable"); row.put("event_id", 0);
        if (!detail.isEmpty()) row.put("detail", detail);
        return row;
    }

    private static void observe(Writer sink) throws Exception {
        Properties hashes = new Properties();
        try (InputStream in = EventProbe.class.getResourceAsStream("/bcs-event-fingerprints.properties")) {
            if (in == null) throw new IllegalStateException("Missing build fingerprints");
            hashes.load(in);
        }
        Thread target = null;
        Binding binding = null;
        String bindingStatus = "waiting_for_bcs";
        String detail = "";
        Object lastPopup = null;
        int lastEvent = 0;
        long instance = 0;
        long nextDiscovery = 0;
        while (!Thread.currentThread().isInterrupted()) {
            long now = System.currentTimeMillis();
            try {
                if (target != null && !target.isAlive()) {
                    target = null; binding = null; bindingStatus = "waiting_for_bcs";
                    detail = "event_thread_restarted";
                }
                if (target == null && now >= nextDiscovery) {
                    nextDiscovery = now + 2000;
                    // An already-running vendor thread proves its class is initialized.
                    // Do not load or initialize the BCS application from the agent.
                    for (Thread thread : Thread.getAllStackTraces().keySet()) {
                        if (thread.isAlive() && thread.getClass().getName().equals(MANAGER)) {
                            target = thread;
                            try {
                                verifyClasses(thread.getClass().getClassLoader(), hashes);
                                binding = new Binding(thread.getClass());
                                bindingStatus = "compatible"; detail = "";
                            } catch (Throwable mismatch) {
                                bindingStatus = "unsupported_bcs_build";
                                detail = mismatch.getClass().getSimpleName();
                                binding = null;
                            }
                            break;
                        }
                    }
                }
                Map<String,Object> row = new LinkedHashMap<String,Object>();
                row.put("schema", 1); row.put("probe_version", VERSION);
                row.put("pid", sink.pid); row.put("status", bindingStatus);
                if (!detail.isEmpty()) row.put("detail", detail);
                Object popup = null;
                int event = 0;
                if (binding != null && target != null) {
                    popup = binding.popup.get(target);
                    int managerEvent = binding.event.getInt(target);
                    int service = binding.service.getByte(target) & 255;
                    long readyAt = binding.readyAt.getLong(target);
                    row.put("manager_event_id", managerEvent);
                    row.put("requested_service_id", service);
                    row.put("requested_service", serviceName(service));
                    row.put("service_ready_at_unix_ms", readyAt);
                    row.put("service_delay_elapsed", readyAt > 0 && now >= readyAt);
                    if (popup != null && popup.getClass().getName().equals(POPUP)) {
                        Field id = field(popup.getClass(), "l", int.class);
                        Field countdown = field(popup.getClass(), "n", AtomicInteger.class);
                        Field completed = field(popup.getClass(), "p", AtomicBoolean.class);
                        event = id.getInt(popup);
                        boolean done = ((AtomicBoolean) completed.get(popup)).get();
                        row.put("popup_countdown", ((AtomicInteger) countdown.get(popup)).get());
                        row.put("popup_completed_flag", done);
                        row.put("phase", done && managerEvent == 0 ? "completion_observed"
                            : service > 0 ? "help_requested" : "event_visible");
                    } else {
                        row.put("phase", popup == null ? "idle" : "unsupported_event_mode");
                    }
                    // Skip torn frames while JavaFX swaps the active popup.
                    if (binding.popup.get(target) != popup) { Thread.sleep(POLL_MS); continue; }
                } else row.put("phase", "unavailable");
                row.put("event_id", event);
                row.put("event_kind", eventName(event));
                boolean started = event > 0 && (event != lastEvent || popup != lastPopup);
                if (started) instance++;
                row.put("event_instance", event > 0 ? instance : 0);
                if (lastEvent > 0 && (event != lastEvent || popup != lastPopup)) {
                    row.put("previous_event_id", lastEvent);
                    row.put("previous_event_instance", instance - (started ? 1 : 0));
                }
                String transition = started ? "event_started"
                    : lastEvent > 0 && event == 0 ? "event_no_longer_visible" : "state_changed";
                sink.emit(row, transition, now);
                lastEvent = event; lastPopup = popup;
            } catch (Throwable unavailable) {
                Map<String,Object> row = new LinkedHashMap<String,Object>();
                row.put("schema", 1); row.put("probe_version", VERSION); row.put("pid", sink.pid);
                row.put("status", "capture_error"); row.put("detail", unavailable.getClass().getSimpleName());
                row.put("phase", "unavailable"); row.put("event_id", 0);
                try { sink.emit(row, "status", now); } catch (Throwable ignored) { }
            }
            Thread.sleep(POLL_MS);
        }
    }

    private static final class Binding {
        final Field event, service, readyAt, popup;
        Binding(Class<?> type) throws Exception {
            event = field(type, "j", int.class);
            service = field(type, "l", byte.class);
            readyAt = field(type, "m", long.class);
            popup = type.getDeclaredField("h"); popup.setAccessible(true);
            if (!popup.getType().getName().equals("o.e")) throw new IllegalStateException("Popup API changed");
        }
    }

    private static Field field(Class<?> type, String name, Class<?> wanted) throws Exception {
        Field result = type.getDeclaredField(name);
        if (result.getType() != wanted) throw new IllegalStateException("Field API changed");
        result.setAccessible(true); return result;
    }

    private static void verifyClasses(ClassLoader loader, Properties hashes) throws Exception {
        Map<String,String> actual = new LinkedHashMap<String,String>();
        for (String name : new String[]{"system/q.class", "o/j.class", "l/t.class"}) {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            try (InputStream in = loader.getResourceAsStream(name)) {
                if (in == null) throw new IllegalStateException("Class resource missing");
                byte[] buffer = new byte[8192]; int length;
                while ((length = in.read(buffer)) != -1) digest.update(buffer, 0, length);
            }
            StringBuilder value = new StringBuilder();
            for (byte b : digest.digest()) value.append(String.format(Locale.ROOT, "%02x", b & 255));
            actual.put(name, value.toString());
        }
        int profiles = Integer.parseInt(hashes.getProperty("profiles", "0"));
        for (int profile = 0; profile < profiles; profile++) {
            boolean matches = true;
            for (Map.Entry<String,String> entry : actual.entrySet()) {
                if (!entry.getValue().equals(hashes.getProperty("profile." + profile + "." + entry.getKey()))) {
                    matches = false; break;
                }
            }
            if (matches) return;
        }
        throw new IllegalStateException("Unrecognized BCS event code");
    }

    private static String serviceName(int value) {
        switch (value) {
            case 0: return "none";
            case 2: return "police";
            case 3: return "medical";
            case 5: return "fire_department";
            case 6: return "dispatch";
            case 7: return "phone_number";
            default: return "unknown";
        }
    }

    private static String eventName(int value) {
        switch (value) {
            case 0: return "none";
            case 1: return "heart_attack";
            case 2: return "stroke";
            case 3: return "circulatory_collapse";
            case 4: return "fainted_passenger";
            case 6: return "drunk_passengers";
            case 7: return "disruptive_passenger";
            case 30: return "bin_fire";
            case 31: return "smoke_in_bus";
            case 32: return "oil_spill";
            case 33: return "radio_contest";
            case 34: return "passenger_count_request";
            default: return "unknown";
        }
    }

    private static final class Writer {
        final String pid;
        final Path current, log;
        String last = ""; long lastWrite;
        Writer(Path directory) throws Exception {
            String runtime = ManagementFactory.getRuntimeMXBean().getName();
            String candidate = runtime.split("@", 2)[0];
            pid = candidate.matches("[0-9]+") ? candidate : "unknown";
            Files.createDirectories(directory);
            current = directory.resolve("bcs-events-" + pid + ".json");
            log = directory.resolve("bcs-events-" + pid + ".ndjson");
        }
        void emit(Map<String,Object> state, String transition, long now) throws Exception {
            String signature = json(state);
            boolean changed = !signature.equals(last);
            if (!changed && now - lastWrite < HEARTBEAT_MS) return;
            Map<String,Object> record = new LinkedHashMap<String,Object>(state);
            record.put("transition", changed ? transition : "heartbeat");
            record.put("observed_at", Instant.ofEpochMilli(now).toString());
            String text = json(record) + "\n";
            Path temporary = current.resolveSibling(current.getFileName().toString() + ".tmp");
            Files.write(temporary, text.getBytes(StandardCharsets.UTF_8));
            try { Files.move(temporary, current, StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE); }
            catch (AtomicMoveNotSupportedException unsupported) { Files.move(temporary, current, StandardCopyOption.REPLACE_EXISTING); }
            Files.write(log, text.getBytes(StandardCharsets.UTF_8), StandardOpenOption.CREATE, StandardOpenOption.APPEND);
            last = signature; lastWrite = now;
        }
    }

    private static String json(Map<String,Object> map) {
        StringBuilder out = new StringBuilder("{");
        boolean first = true;
        for (Map.Entry<String,Object> item : map.entrySet()) {
            if (!first) out.append(','); first = false;
            quote(out, item.getKey()); out.append(':');
            Object value = item.getValue();
            if (value instanceof Number || value instanceof Boolean) out.append(value);
            else quote(out, String.valueOf(value));
        }
        return out.append('}').toString();
    }
    private static void quote(StringBuilder out, String value) {
        out.append('"');
        for (int i = 0; i < value.length(); i++) {
            char c = value.charAt(i);
            switch (c) {
                case '"': out.append("\\\""); break;
                case '\\': out.append("\\\\"); break;
                case '\n': out.append("\\n"); break;
                case '\r': out.append("\\r"); break;
                case '\t': out.append("\\t"); break;
                default: if (c < 32) out.append(String.format(Locale.ROOT, "\\u%04x", (int)c)); else out.append(c);
            }
        }
        out.append('"');
    }
}
