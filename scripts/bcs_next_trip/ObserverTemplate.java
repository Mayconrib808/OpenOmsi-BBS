package bridge;

/** Adds selected-trip metadata to the existing continuation log event. */
public final class ObserverTemplate {
    private static String bridgeNextTripDetails(String text) {
        if (text == null || !(text.endsWith("Weitere Tour Multiplayer") ||
                text.endsWith("Weitere Event Tour Multiplayer"))) return text;
        // Reading the accepted selection is observational. Do not select a trip,
        // call a web service, edit an evaluation, or start/stop the game here.
        try {
            // Internal vehicle/configuration flags are obfuscated and change
            // meaning between BCS builds. An accepted continuation already
            // supplies its multiplayer mode and complete selection.
            if (system.I.h() != 2 || system.I.I() <= 0 ||
                    system.I.i() == null || system.I.j() == null) return text;
            return text + "\n[OpenOmsi-BBS next-trip log v2]\nSchicht ID: " +
                system.I.I() + "\nKarte: " + system.I.i().e() +
                "\nTour: " + system.I.j().c();
        } catch (Throwable unavailable) {
            // A logging extension must never interrupt BCS or change its result.
            return text;
        }
    }
}
