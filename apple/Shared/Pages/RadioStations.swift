import SwiftUI

struct RadioStations: View {
    @Environment(LauncherStore.self) private var store
    @State private var rows: [Row] = []
    @State private var loaded = false
    @State private var pending: Task<Void, Never>?

    struct Row: Identifiable, Equatable {
        var id = UUID()
        var name: String
        var address: String
    }

    private struct Station: Decodable {
        var name: String
        var address: String
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            if loaded {
                let numbers = numbering
                ForEach($rows) { $row in
                    HStack(spacing: 8) {
                        Text(numbers[row.id] ?? "").font(.inter(12).monospacedDigit()).foregroundStyle(Theme.muted).frame(width: 18, alignment: .trailing)
                        InputField(prompt: "Name", text: $row.name, onSubmit: flush).frame(maxWidth: 170)
                        InputField(prompt: "Address", text: $row.address, onSubmit: flush)
                        Button { remove(row.id) } label: { Image(systemName: "trash") }
                            .buttonStyle(.launcher(.ghost, small: true))
                            .help(Text("Remove this station"))
                            .accessibilityLabel(Text("Remove this station"))
                    }
                }
                HStack {
                    Button { rows.append(Row(name: "", address: "")) } label: { Label("Add a station", systemImage: "plus") }
                        .buttonStyle(.launcher(.plain, small: true))
                    Spacer()
                }
            }
        }
        .onAppear(perform: load)
        .onDisappear { if pending != nil { flush() } }
        .onChange(of: rows) { old, new in
            guard loaded, old.map(\.id) == new.map(\.id) else { return }
            pending?.cancel()
            pending = Task { @MainActor in
                try? await Task.sleep(for: .milliseconds(400))
                if !Task.isCancelled { flush() }
            }
        }
    }

    private var numbering: [UUID: String] {
        var n = 0
        var out: [UUID: String] = [:]
        for r in rows where !(r.address.split(separator: "|", omittingEmptySubsequences: false).first ?? "").trimmingCharacters(in: .whitespaces).isEmpty {
            n += 1
            out[r.id] = "\(n)"
        }
        return out
    }

    private func load() {
        guard !loaded else { return }
        do {
            rows = try Bridge.api([Station].self, "radio_stations").map { Row(name: $0.name, address: $0.address) }
        } catch {
            store.say(error.localizedDescription, error: true)
        }
        loaded = true
    }

    private func remove(_ id: UUID) {
        rows.removeAll { $0.id == id }
        flush()
    }

    private func flush() {
        pending?.cancel()
        pending = nil
        guard loaded else { return }
        do {
            _ = try Bridge.api("radio_stations_save", ["list": rows.map { ["name": $0.name, "address": $0.address] }])
        } catch {
            store.say(error.localizedDescription, error: true)
        }
    }
}
