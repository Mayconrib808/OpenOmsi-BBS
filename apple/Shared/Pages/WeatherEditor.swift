import SwiftUI

struct CustomWeather: Codable {
    var visibility_m: Double
    var brightness: Double
    var wind_dir: Double
    var wind_speed: Double
    var temp_c: Double
    var humidity: Double
    var pressure: Double
    var cloud_cover: Double
    var cloud_base_m: Double
    var precip: Int
    var precip_intensity: Double
    var road_wetness: Double
    var snow_cover: Bool
    var snow_on_road: Bool
}

@MainActor
enum WeatherChoice {
    private static var nearest: [String: String] = [:]
    private static var airportList: [(value: String, label: String)]?

    static func isCustom(_ w: String) -> Bool { w.lowercased().hasPrefix("custom:") }

    static func editable(_ w: String) -> Bool { isCustom(w) || w.lowercased().hasSuffix(".owt") }

    static func nearestAirport(_ map: String) -> String {
        if map.isEmpty { return "" }
        if let hit = nearest[map] { return hit }
        guard let code = try? Bridge.api(String.self, "nearest_airport", ["map": map]) else { return "" }
        nearest[map] = code
        return code
    }

    static func metar(_ choice: Choice) -> String {
        let w = choice.weather
        if w.hasPrefix("metar:"), w.count > 6 { return String(w.dropFirst(6)) }
        return nearestAirport(choice.map)
    }

    static func airports() -> [(value: String, label: String)] {
        if let a = airportList { return a }
        let a = ((try? Bridge.api([[String]].self, "metar_airports")) ?? []).compactMap { p in p.count == 2 ? (value: p[0], label: p[1]) : nil }
        if !a.isEmpty { airportList = a }
        return a
    }

    static func metarText(_ code: String) -> String {
        String(localized: "METAR of {} (fetched at the start)").replacingOccurrences(of: "{}", with: code)
    }
}

struct AirportField: View {
    @Environment(LauncherStore.self) private var store
    var code: String
    @State private var text = ""

    var body: some View {
        let list = WeatherChoice.airports()
        HStack(spacing: 8) {
            InputField(prompt: "ICAO", text: $text, mono: true) { use(text) }
                .frame(width: 96)
                .onAppear { text = code }
                .onChange(of: code) { _, v in text = v }
                .onChange(of: text) { _, v in if clean(v).count == 4, clean(v) != code { use(v) } }
            if !list.isEmpty {
                SelectField(options: list.contains { $0.value == code } ? list : [(value: code, label: code)] + list, selection: code) {
                    store.act("weather", ["weather": "metar:\($0)"])
                }
            }
        }
    }

    private func clean(_ v: String) -> String { String(v.uppercased().filter { $0.isLetter || $0.isNumber }) }

    private func use(_ v: String) {
        let c = clean(v)
        if c.count == 4 { store.act("weather", ["weather": "metar:\(c)"]) } else { text = code }
    }
}

struct WeatherEditor: View {
    @Environment(LauncherStore.self) private var store
    @Environment(\.dismiss) private var dismiss
    var from: String
    @State private var w: CustomWeather?
    @State private var busy = false

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Text("Custom weather").font(.inter(17, .semibold))
                Spacer()
                Button { dismiss() } label: { Image(systemName: "xmark") }.buttonStyle(.ghost).help("Close")
            }
            .padding(.horizontal, 20).padding(.top, 16).padding(.bottom, 10)
            Scroll {
                if let w {
                    fields(w).padding(.horizontal, 20).padding(.bottom, 12)
                } else {
                    ProgressView().frame(maxWidth: .infinity).padding(40)
                }
            }
            HStack(spacing: 8) {
                Spacer()
                Button("Cancel") { dismiss() }.buttonStyle(.launcher(.ghost))
                Button("Done") { use() }.buttonStyle(.launcher(.primary)).disabled(w == nil || busy)
            }
            .padding(.horizontal, 20).padding(.vertical, 14)
        }
        .background(Theme.surface.ignoresSafeArea())
        #if os(macOS)
        .frame(minWidth: 620, idealWidth: 680, minHeight: 520)
        #endif
        .task {
            do {
                w = try Bridge.api(CustomWeather.self, "custom_weather", ["weather": from])
            } catch {
                store.say(error.localizedDescription, error: true)
                dismiss()
            }
        }
    }

    private func fields(_ v: CustomWeather) -> some View {
        LazyVGrid(columns: [GridItem(.adaptive(minimum: 250), spacing: 20, alignment: .top)], alignment: .leading, spacing: 8) {
            slider("Visibility", \.visibility_m, 50 ... 50000, 50, distance)
            slider("Brightness", \.brightness, 0 ... 1.5, 0.05, percent)
            slider("Wind direction", \.wind_dir, 0 ... 355, 5) { "\(Int($0.rounded()))°" }
            slider("Wind speed", \.wind_speed, 0 ... 50, 0.5) { String(format: "%.1f m/s", $0) }
            slider("Temperature", \.temp_c, -40 ... 50, 1) { "\(Int($0.rounded())) °C" }
            slider("Humidity", \.humidity, 0 ... 100, 1) { h in
                String(localized: "{} % | dew point {} °C").replacingFirst("\(Int(h.rounded()))").replacingFirst("\(Int(dewPoint(v.temp_c, h).rounded()))")
            }
            slider("Cloud cover", \.cloud_cover, 0 ... 1, 0.01) { $0 <= 0 ? String(localized: "Clear sky") : percent($0) }
            slider("Wet roads", \.road_wetness, 0 ... 1, 0.05, percent)
            Field("Precipitation") {
                SelectField(options: [("0", "None"), ("1", "Rain"), ("2", "Snow")].map { (value: $0.0, label: NSLocalizedString($0.1, comment: "")) },
                            selection: String(v.precip), onChange: { w?.precip = Int($0) ?? 0 })
            }
            slider("Precipitation strength", \.precip_intensity, 0 ... 255, 1) { "\(Int($0.rounded())) / 255" }
            ToggleRow(label: "Snow cover", isOn: v.snow_cover) { w?.snow_cover = $0 }
            ToggleRow(label: "Snow on road", isOn: v.snow_on_road) { w?.snow_on_road = $0 }
        }
    }

    private func slider(_ label: LocalizedStringKey, _ key: WritableKeyPath<CustomWeather, Double>, _ range: ClosedRange<Double>, _ step: Double, _ format: @escaping (Double) -> String) -> some View {
        SliderField(label: label, value: w?[keyPath: key] ?? range.lowerBound, range: range, step: step, format: format) { w?[keyPath: key] = $0 }
    }

    private func percent(_ v: Double) -> String { "\(Int((v * 100).rounded())) %" }

    private func distance(_ v: Double) -> String {
        v >= 49950 ? String(localized: "Unlimited") : v >= 1000 ? String(format: "%.1f km", v / 1000) : "\(Int(v.rounded())) m"
    }

    private func dewPoint(_ temp: Double, _ rh: Double) -> Double {
        let g = log(min(100, max(0.1, rh)) / 100) + (17.67 * temp) / (243.5 + temp)
        return (243.5 * g) / (17.67 - g)
    }

    private func use() {
        guard let w else { return }
        busy = true
        do {
            let custom = try JSONSerialization.jsonObject(with: JSONEncoder().encode(w))
            let weather = try Bridge.api(String.self, "custom_weather_text", ["custom": custom])
            store.act("weather", ["weather": weather])
            dismiss()
        } catch {
            store.say(error.localizedDescription, error: true)
            busy = false
        }
    }
}

struct DateChooser: View {
    @Environment(LauncherStore.self) private var store
    var done: () -> Void

    private static let calendar: Calendar = {
        var c = Calendar(identifier: .gregorian)
        c.timeZone = TimeZone(identifier: "UTC") ?? .current
        c.firstWeekday = 2
        return c
    }()

    private static func formatter() -> DateFormatter {
        let f = DateFormatter()
        f.dateFormat = "yyyy-MM-dd"
        f.locale = Locale(identifier: "en_US_POSIX")
        f.timeZone = calendar.timeZone
        return f
    }

    var body: some View {
        let f = Self.formatter()
        DatePicker("Date", selection: Binding(get: { f.date(from: store.choice.date) ?? Date() }, set: {
            store.act("date", ["date": f.string(from: $0)])
            done()
        }), displayedComponents: .date)
            .datePickerStyle(.graphical)
            .labelsHidden()
            .environment(\.calendar, Self.calendar)
            .environment(\.timeZone, Self.calendar.timeZone)
            .tint(Theme.accent)
            .padding(12)
            .frame(minWidth: 300)
    }
}

extension View {
    func launcherSheet(_ store: LauncherStore) -> some View {
        environment(store)
            .font(.inter(14))
            .foregroundStyle(Theme.text)
            .tint(Theme.accent)
            .preferredColorScheme(.dark)
    }
}

private extension String {
    func replacingFirst(_ value: String) -> String {
        guard let r = range(of: "{}") else { return self }
        return replacingCharacters(in: r, with: value)
    }
}
