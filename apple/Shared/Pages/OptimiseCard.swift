import SwiftUI

struct OptimiseBlock: View {
    @Environment(LauncherStore.self) private var store
    @State private var status: JSON?
    @State private var target = "0"
    @State private var busy = false

    private static let targets = [("0", "Automatic"), ("60", "60 fps"), ("90", "90 fps"), ("120", "120 fps"), ("144", "144 fps")]

    var body: some View {
        let armed = (status?["armed"]).map { $0 != .null } ?? false
        let gameRuns = store.snapshot.in_game || !store.snapshot.instances.isEmpty
        VStack(alignment: .leading, spacing: 10) {
            SectionTitle("Optimise for my PC")
            Text("During your next drive the game measures for about a minute and adjusts the graphics to reach the frame rate you aim at. Your settings are saved first.")
                .font(.inter(12.5)).foregroundStyle(Theme.muted).fixedSize(horizontal: false, vertical: true)
            LabeledRow("Frame rate aimed at", detail: NSLocalizedString("Automatic: the screen's refresh rate, at most 100 fps", comment: "")) {
                SelectField(options: targetOptions, selection: target, onChange: { target = $0 })
                    .disabled(busy || armed)
            }
            if armed {
                HStack {
                    Text("Waiting for your next drive").font(.inter(13)).foregroundStyle(Theme.muted)
                    Spacer()
                    Button { act("optimise_cancel") } label: { Label("Cancel", systemImage: "xmark") }
                        .buttonStyle(.secondary).disabled(busy)
                }
            } else {
                HStack {
                    Button { act("optimise_arm", ["target": Int(target) ?? 0]) } label: { Label("Optimise on my next drive", systemImage: "slider.horizontal.3") }
                        .buttonStyle(.primary).disabled(busy || status == nil)
                    Spacer()
                }
            }
            if let last = status?["last"], last != .null {
                result(last)
            }
            if status?["backup"]?.bool == true {
                HStack {
                    Button(action: restore) { Label("Restore my previous settings", systemImage: "clock.arrow.circlepath") }
                        .buttonStyle(.secondary).disabled(busy || gameRuns)
                    Spacer()
                }
            }
        }
        .padding(.bottom, 4)
        .task {
            while !Task.isCancelled {
                load()
                try? await Task.sleep(for: .seconds(5))
            }
        }
    }

    private var targetOptions: [(value: String, label: String)] {
        let list = Self.targets.contains { $0.0 == target } ? Self.targets : Self.targets + [(target, "\(target) fps")]
        return list.map { (value: $0.0, label: NSLocalizedString($0.1, comment: "")) }
    }

    private func result(_ last: JSON) -> some View {
        let fps = { (k: String) in String(format: "%.0f", last[k]?.double ?? 0) }
        let changes: [JSON] = if case .array(let a) = last["changes"] ?? .null { a } else { [] }
        return VStack(alignment: .leading, spacing: 4) {
            Text(NSLocalizedString("Last optimisation: {before} → {after} fps", comment: "").replacingOccurrences(of: "{before}", with: fps("before_fps")).replacingOccurrences(of: "{after}", with: fps("after_fps")))
                .font(.inter(13)).foregroundStyle(Theme.text)
            if changes.isEmpty {
                Text("Nothing needed changing").font(.inter(12.5)).foregroundStyle(Theme.muted)
            } else {
                ForEach(changes.indices, id: \.self) { i in
                    let key = changes[i]["key"]?.string ?? ""
                    Text(verbatim: "\(label(key)): \(shown(key, changes[i]["from"])) → \(shown(key, changes[i]["to"]))")
                        .font(.inter(12.5)).foregroundStyle(Theme.muted)
                }
            }
            if last["restart"]?.bool == true {
                Text("Takes effect when the game starts the next time").font(.inter(12.5)).foregroundStyle(Theme.muted)
            }
        }
    }

    private func label(_ key: String) -> String {
        Choices.rows[key].map { NSLocalizedString($0.label, comment: "") } ?? key
    }

    private func shown(_ key: String, _ v: JSON?) -> String {
        guard let v else { return "" }
        if case .bool(let b) = v { return b ? String(localized: "On") : String(localized: "Off") }
        let s = v.string ?? ""
        let hit = Choices.rows[key]?.options.first { o in o.0 == s || (Double(o.0) != nil && Double(o.0) == v.double && !s.isEmpty) }
        return hit.map { NSLocalizedString($0.1, comment: "") } ?? s
    }

    private func load() {
        do {
            let st = try Bridge.api(JSON.self, "optimise_status")
            status = st
            if let a = st["armed"]?.string { target = a }
        } catch {
            if status == nil { status = .object([:]) }
        }
    }

    private func act(_ cmd: String, _ args: [String: Any] = [:]) {
        busy = true
        do {
            status = try Bridge.api(JSON.self, cmd, args)
        } catch {
            store.say(error.localizedDescription, error: true)
        }
        busy = false
    }

    private func restore() {
        busy = true
        do {
            let settings = try Bridge.api(JSON.self, "optimise_restore")
            if case .object(let o) = settings {
                store.act("settings", ["values": o.mapValues(\.any), "replace": true])
            }
            status = try Bridge.api(JSON.self, "optimise_status")
            store.say(String(localized: "Your previous settings are back"), error: false)
        } catch {
            store.say(error.localizedDescription, error: true)
        }
        busy = false
    }
}
