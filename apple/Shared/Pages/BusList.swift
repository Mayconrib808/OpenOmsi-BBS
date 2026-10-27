import SwiftUI

struct BusFamily: Identifiable {
    var key: String
    var name: String
    var maker: String
    var versions: [(v: VehicleInfo, label: String)]
    var id: String { key }
}

func busFamilies(_ list: [VehicleInfo]) -> [BusFamily] {
    var order: [String] = []
    var by: [String: [VehicleInfo]] = [:]
    for v in list {
        let f = favKey(v.file)
        let dir = f.lastIndex(of: "/").map { String(f[...$0]) } ?? ""
        let key = "\(v.manufacturer.lowercased())|\(dir)"
        if by[key] == nil { order.append(key) }
        by[key, default: []].append(v)
    }
    let words = { (s: String) in s.replacingOccurrences(of: "_", with: " ").split(whereSeparator: \.isWhitespace).map(String.init) }
    let wordy = { (s: String) in s.contains { $0.isLetter || $0.isNumber } }
    return order.map { key in
        let vs = by[key] ?? []
        let types = vs.map { words($0.type_name.isEmpty ? $0.name : $0.type_name) }
        var n = 0
        while types.allSatisfy({ n < $0.count && $0[n].lowercased() == types[0][n].lowercased() }) { n += 1 }
        if vs.count > 1 && types.contains(where: { $0.count == n }) { n = max(0, n - 1) }
        while n > 0 && !wordy(types[0][n - 1]) { n -= 1 }
        let folder = vs[0].file.replacingOccurrences(of: "\\", with: "/").split(separator: "/").dropLast().last.map { $0.replacingOccurrences(of: "_", with: " ") } ?? vs[0].name
        let name = vs.count == 1 ? vs[0].name : n > 0 ? types[0][..<n].joined(separator: " ") : folder
        let rest = { (w: [String]) -> String in
            var i = n
            while i < w.count - 1 && !wordy(w[i]) { i += 1 }
            return w[min(i, w.count)...].joined(separator: " ")
        }
        let versions = vs.indices.map { i in (v: vs[i], label: vs.count > 1 && n > 0 ? rest(types[i]) : (vs[i].type_name.isEmpty ? vs[i].name : vs[i].type_name)) }
            .sorted { $0.label.localizedStandardCompare($1.label) == .orderedAscending }
        return BusFamily(key: key, name: name, maker: vs[0].manufacturer, versions: versions)
    }
    .sorted { a, b in
        a.maker != b.maker ? a.maker.localizedCompare(b.maker) == .orderedAscending : a.name.localizedStandardCompare(b.name) == .orderedAscending
    }
}

func filterBuses(_ all: [VehicleInfo], search: String, show: String, maker: String = "", starred: (String) -> Bool) -> [VehicleInfo] {
    let q = search.trimmingCharacters(in: .whitespaces)
    return all
        .filter { maker.isEmpty || $0.manufacturer == maker }
        .filter { show != "fav" || starred($0.file) }
        .filter { show != "mods" || $0.installed == true }
        .filter { q.isEmpty || "\($0.name) \($0.manufacturer) \($0.type_name) \($0.folder)".localizedCaseInsensitiveContains(q) }
}

struct BusTree: View {
    @Environment(LauncherStore.self) private var store
    var list: [VehicleInfo]
    var searching: Bool
    var big = false
    var onPick: (VehicleInfo) -> Void
    @State private var open: Set<String> = []

    var body: some View {
        let fams = busFamilies(list)
        let chosen = store.choice.bus
        LazyVStack(spacing: big ? 6 : 0) {
            ForEach(fams) { f in
                if f.versions.count == 1 {
                    let v = f.versions[0].v
                    row(v, title: v.type_name.isEmpty ? v.name : v.type_name, sub: [v.manufacturer, paints(v)].filter { !$0.isEmpty }.joined(separator: " - "), indent: false)
                } else {
                    let shown = searching || open.contains(f.key)
                    let has = f.versions.contains { $0.v.file == chosen }
                    Button {
                        if open.contains(f.key) { open.remove(f.key) } else { open.insert(f.key) }
                        Haptics.tap()
                    } label: {
                        HStack(spacing: 10) {
                            VStack(alignment: .leading, spacing: 2) {
                                Text(f.name).font(.inter(big ? 15 : 14, .semibold)).foregroundStyle(Theme.text).lineLimit(1)
                                Text([f.maker, tr("{n} versions", ["n": "\(f.versions.count)"])].filter { !$0.isEmpty }.joined(separator: " - "))
                                    .font(.inter(12.5)).foregroundStyle(Theme.muted).lineLimit(1)
                            }
                            Spacer(minLength: 6)
                            Image(systemName: shown ? "chevron.down" : "chevron.right").font(.system(size: 12, weight: .semibold)).foregroundStyle(Theme.muted)
                        }
                        .modifier(BusRowLook(on: has && !shown, big: big))
                    }
                    .buttonStyle(.plain)
                    if shown {
                        ForEach(f.versions, id: \.v.file) { x in
                            row(x.v, title: x.label, sub: paints(x.v), indent: true)
                        }
                    }
                }
            }
        }
        .onAppear {
            if let f = fams.first(where: { $0.versions.count > 1 && $0.versions.contains { $0.v.file == chosen } }) { open.insert(f.key) }
        }
    }

    private func paints(_ v: VehicleInfo) -> String {
        v.paints.isEmpty ? "" : tr("{n} paints", ["n": "\(v.paints.count + 1)"])
    }

    private func row(_ v: VehicleInfo, title: String, sub: String, indent: Bool) -> some View {
        HStack(spacing: 4) {
            Button { onPick(v); Haptics.tap() } label: {
                HStack(spacing: 10) {
                    VStack(alignment: .leading, spacing: 2) {
                        Text(title).font(.inter(big ? 15 : 14, .medium)).foregroundStyle(Theme.text).lineLimit(1)
                        if !sub.isEmpty { Text(sub).font(.inter(12.5)).foregroundStyle(Theme.muted).lineLimit(1) }
                    }
                    Spacer(minLength: 6)
                    if store.snapshot.fresh.contains(v.file) { Tag(text: "New", kind: .accent) }
                    if let m = v.missing_packs, !m.isEmpty {
                        Image(systemName: "exclamationmark.triangle.fill").font(.system(size: 13)).foregroundStyle(Theme.warn).help("Parts missing")
                    }
                    if big && v.file == store.choice.bus { Image(systemName: "checkmark").font(.system(size: 15, weight: .bold)).foregroundStyle(Theme.accent) }
                }
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            StarButton(file: v.file)
        }
        .padding(.leading, indent ? 18 : 0)
        .modifier(BusRowLook(on: v.file == store.choice.bus, big: big))
    }
}

private struct BusRowLook: ViewModifier {
    var on: Bool
    var big: Bool

    func body(content: Content) -> some View {
        content
            .padding(.leading, 12).padding(.trailing, 6).padding(.vertical, big ? 8 : 6)
            .frame(maxWidth: .infinity, minHeight: big ? 56 : 46, alignment: .leading)
            .background(on ? Theme.accentSoft : (big ? Theme.surface : .clear), in: RoundedRectangle(cornerRadius: big ? 12 : Theme.radiusSm, style: .continuous))
            .contentShape(Rectangle())
    }
}
