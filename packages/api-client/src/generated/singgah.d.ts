/* eslint-disable */
// AUTO-GENERATED from contracts/openapi/singgah.yaml — do not edit.
// Regenerate with: pnpm client:generate
export interface paths {
    "/health": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Service liveness
         * @description Returns 200 while the API process is serving requests.
         */
        get: operations["getHealth"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/ready": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Readiness probe
         * @description 200 when dependencies are usable (today: database). 503 with the error envelope when the database is unconfigured or unreachable.
         */
        get: operations["getReady"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/version": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Build version
         * @description Returns the API build version injected at build time.
         */
        get: operations["getVersion"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/stations": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * List and search stations and stops
         * @description Three modes: with `bbox` returns stops inside the viewport (minLon,minLat,maxLon,maxLat — the map's scoped fetch); with `query` alone, case-insensitive search over display name, station code, and official name; with neither, the unfiltered reference list. `bbox` and `query` combine as "search within the viewport".
         */
        get: operations["listStations"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/stations/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Station detail
         * @description One canonical station/stop with the routes serving it and its internal transfers, plus ingest provenance.
         */
        get: operations["getStation"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/stations/{id}/departures": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Station departures board
         * @description Scheduled departures from one station, grouped by line and direction (boundFor). Source is the in-house schedule snapshot (GTFS + reconstructed timetables) — always "scheduled", never live; per-departure `estimated` flags headway/derived times. Times are Asia/Jakarta wall clock; each direction also carries the most recent past departure when one falls inside the lookback window.
         */
        get: operations["getStationDepartures"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/routes": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * List transit routes
         * @description Routes across all agencies; `query` filters short/long name case-insensitively. Without a query returns up to `limit` routes.
         */
        get: operations["listRoutes"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/routes/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Route detail
         * @description One canonical route with the stops it serves, in the provider's declared segment order. A route with no ingested sequence returns an empty list.
         */
        get: operations["getRoute"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/providers": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * List data providers with ingest freshness
         * @description The provider registry as a health surface: `lastSuccessAt` is the stamp of the most recent successful ingest — its absence means the provider is registered but has never been ingested. `lastAttemptAt` marks the most recent run start; an attempt newer than the last success means the latest ingest failed (or is still running). License, attribution, and known limitations travel with the registry (docs/35, docs/37).
         */
        get: operations["listProviders"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/map/lines": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Route path geometries for the network map
         * @description Drawable geometry for every route touching the viewport, returned as a GeoJSON FeatureCollection of MultiLineString features. Real ingested path geometry (GTFS shapes, OSM rail relations) wins; routes with no ingested shape fall back to polylines through their ordered stops — `properties.source` on each feature says which it is ('shape' or 'stops'). Display geometry only; it carries no schedule or realtime meaning.
         */
        get: operations["listRouteLines"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/journeys": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Station-to-station itineraries
         * @description Itineraries between two canonical station UUIDs, computed by the in-house schedule planner over the ingested timetable snapshot. `itineraries` is a (possibly empty) list — empty means no plan exists for the filters, never an error. Each itinerary is labelled by the trade-off it wins (fastest / fewest_transfers / least_walking / alternative). Leg times are timetable values; legs built on frequency templates or reconstructed stop_times mark the itinerary `status: "estimated"`. `fareReference` is the provider's corridor fare estimate — optional context that may be absent on upstream outage. No realtime exists yet.
         */
        get: operations["planJourney"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/auth/session": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /**
         * Create anonymous session
         * @description Mints a fresh anonymous user + session (ADR-010). No registration or personal data — the returned token IS the account credential; losing it loses the passport. Rate-limited per client IP.
         */
        post: operations["createSession"];
        /**
         * Revoke current session
         * @description Invalidates the bearer token used for this request ("logout").
         */
        delete: operations["revokeSession"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/auth/account": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        /**
         * Delete account and all owned data
         * @description Deletes the user row; FK cascades permanently remove sessions, visit events, and journal entries (docs/26 delete path).
         */
        delete: operations["deleteAccount"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/visits": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * List own visit events
         * @description Returns the authenticated user's check-ins, newest first.
         */
        get: operations["listVisits"];
        put?: never;
        /**
         * Check in at a station/stop
         * @description Records a visit event for the authenticated user (docs/16). Send `lat` + `lon` together for a geofence-verified check-in (confirmed when within ~200 m); omit them for a manual, unverified check-in. Either way the visit is recorded — confidence is marked, never faked. Safe to replay: `clientMutationId` dedupes offline-queue retries. Rate-limited per IP.
         */
        post: operations["checkin"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/passport/progress": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Passport progress
         * @description Distinct catalog stops the user has visited over stops the network serves — overall, per mode, and per corridor (route). Low-confidence visits count toward progress; passport progress is personal (docs/42). Geography and collection breakdowns land with their own features.
         */
        get: operations["getPassportProgress"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/journal": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * List own journal entries
         * @description Newest first, capped by `limit`.
         */
        get: operations["listJournalEntries"];
        put?: never;
        /**
         * Create journal entry
         * @description Private note linked optionally to a stop and/or one of the user's own visit events (docs/16). `visitEventId` referencing another user's visit answers 404 — foreign ids are never revealed.
         */
        post: operations["createJournalEntry"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/journal/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        /** Delete journal entry */
        delete: operations["deleteJournalEntry"];
        options?: never;
        head?: never;
        /**
         * Update journal entry
         * @description Last-write-wins by default. Send `baseUpdatedAt` (the `updatedAt` the client read) for an optimistic version check — a mismatch answers 409 instead of overwriting (docs/24).
         */
        patch: operations["updateJournalEntry"];
        trace?: never;
    };
    "/api/v1/collections": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Published collections
         * @description Curated editorial stop sets that passport progress can be measured against (docs/42). `kind` keeps the docs/13 distinction explicit — 'curated' sets are editor-picked, 'algorithmic' ones must never masquerade as curation. Draft collections never appear here.
         */
        get: operations["listCollections"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
}
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
        ErrorEnvelope: components["schemas"]["error"];
        error: {
            error: {
                /**
                 * @description Machine-readable error code, e.g. NOT_FOUND, PROVIDER_STALE
                 * @example NOT_FOUND
                 */
                code: string;
                /** @description Human-readable explanation of the actual degraded state */
                message: string;
                /** @description Correlates with the X-Request-ID response header */
                requestId?: string;
                details?: {
                    [key: string]: unknown;
                };
            };
        };
        RouteRef: {
            /** Format: uuid */
            id: string;
            shortName?: string;
            longName?: string;
            /** @enum {string} */
            mode: "rail" | "subway" | "tram" | "bus" | "ferry" | "other";
            /** @description Hex color without leading '#' */
            color?: string;
            agencyCode?: string;
            agencyName?: string;
        };
        StationSummary: {
            /**
             * Format: uuid
             * @description Canonical Singgah UUID
             */
            id: string;
            name: string;
            /** @description Public station code, e.g. SUD */
            code?: string;
            /** @enum {string} */
            kind: "station" | "stop" | "platform" | "entrance";
            lat: number;
            lon: number;
            providerCode: string;
            /** @description Provider-declared operator code (e.g. TJ, MRTJ, KCI, LRTJ, LRTJBDB). Empty when the provider does not survey it — clients key official brand marks off this field. */
            operator?: string;
            /** @description Catalog routes serving this stop — populated on text search (combobox badges); absent on bbox and reference listings. */
            lines?: components["schemas"]["RouteRef"][];
        };
        /** @description Lightweight stop reference embedded in other resources. */
        StopRef: {
            /** Format: uuid */
            id: string;
            name: string;
            code?: string;
            lat: number;
            lon: number;
        };
        Transfer: {
            toStop: components["schemas"]["StopRef"];
            walkDistanceM?: number;
            notes?: string;
        };
        /** @description Provider-declared amenity. accessibilityRelevant classifies the type only — presence says nothing about whether the facility works today (docs/41). */
        Facility: {
            /** @description Canonical lowercase amenity type, e.g. toilet, elevator_paid, toilet_accessible */
            type: string;
            /** @description Provider location note, e.g. "Concourse" */
            text?: string;
            accessibilityRelevant?: boolean;
        };
        /** @description Provenance of the upstream provider row. */
        SourceMeta: {
            /** @description Canonical provider code, e.g. commute */
            provider: string;
            /**
             * Format: date-time
             * @description When the provider row was ingested
             */
            fetchedAt?: string;
            /**
             * Format: date-time
             * @description Upstream update timestamp reported by the provider
             */
            sourceUpdatedAt?: string;
        };
        StationDetail: components["schemas"]["StationSummary"] & {
            officialName?: string;
            lines: components["schemas"]["RouteRef"][];
            transfers: components["schemas"]["Transfer"][];
            facilities: components["schemas"]["Facility"][];
            source: components["schemas"]["SourceMeta"];
        };
        Departure: {
            /**
             * @description Asia/Jakarta wall clock, HH:MM
             * @example 07:04
             */
            time: string;
            /** @description Operator trip number — null when the operator publishes none */
            tripNumber: string | null;
            /** @description Terminal station name, matching the leg's headsign */
            boundFor: string;
            /** @description True when the time comes from a headway template (frequency-based service) or a derived row the source never published — ESTIMATED per docs/09, not a scheduled departure. */
            estimated?: boolean;
        };
        /** @description One direction (boundFor) on a line. departures holds upcoming boardings inside the requested window, soonest first; previousDeparture is the latest boarding already gone inside the lookback window, absent when none is seen. */
        DepartureDirection: {
            /** @description Terminal station name published by the provider */
            boundFor: string;
            departures: components["schemas"]["Departure"][];
            previousDeparture?: components["schemas"]["Departure"];
        };
        /** @description Departures on one provider line serving the station. route is the canonical route when the provider line resolves to one — absent when it does not, in which case lineCode is the only identity shown. */
        DepartureLine: {
            /** @description Provider line code, e.g. B — diagnostic identity */
            lineCode: string;
            route?: components["schemas"]["RouteRef"];
            directions: components["schemas"]["DepartureDirection"][];
        };
        /** @description A station's departure board. status is fixed "scheduled" — the source is the in-house schedule snapshot (GTFS + reconstructed timetables), never realtime. Lines and directions are ordered by soonest upcoming departure. */
        StationDepartures: {
            station: components["schemas"]["StopRef"];
            /** @enum {string} */
            status: "scheduled";
            /** @description Effective lookahead window the board was computed with */
            windowMinutes: number;
            lines: components["schemas"]["DepartureLine"][];
            source: {
                /** @description "schedule" — the in-house schedule snapshot */
                provider: string;
                /**
                 * Format: date-time
                 * @description Server time the board was computed — the anchor for "now"
                 */
                requestedAt: string;
                /**
                 * Format: date-time
                 * @description When the planner schedule snapshot was loaded
                 */
                snapshotAt?: string;
            };
        };
        RouteSummary: components["schemas"]["RouteRef"] & {
            providerCode: string;
        };
        /** @description StopRef plus its position on the route. seq is the flattened provider order across segments; segmentKind preserves TRUNK/BRANCH topology. */
        RouteStop: {
            /** Format: uuid */
            id: string;
            name: string;
            code?: string;
            lat: number;
            lon: number;
            /** @description 1-based position across the route's flattened segments */
            seq: number;
            /** @description Operator-facing number when published (M01, C07…) */
            stationNumber?: string;
            /** @description Provider segment kind: TRUNK | BRANCH | upstream value */
            segmentKind?: string;
        };
        RouteDetail: components["schemas"]["RouteSummary"] & {
            /** @description Stops in the provider's declared line order (route_stops.seq). A route with no ingested sequence returns an empty list rather than an invented order. */
            stops: components["schemas"]["RouteStop"][];
            source: components["schemas"]["SourceMeta"];
        };
        /** @description Data-provider registry row with its ingest freshness signal. lastSuccessAt absent means registered but never successfully ingested. */
        Provider: {
            code: string;
            name: string;
            licenseName?: string;
            attributionText?: string;
            allowedUse?: string;
            refreshCadence?: string;
            owner?: string;
            knownLimitations?: string;
            isActive: boolean;
            /** Format: date-time */
            lastSuccessAt?: string;
            /**
             * Format: date-time
             * @description Stamp of the most recent ingest attempt — newer than lastSuccessAt (or success absent) means the last run failed or is still running.
             */
            lastAttemptAt?: string;
        };
        /** @description GeoJSON MultiLineString — the merged path for one route. Real ingested shapes and stop-sequence fallbacks both collect into MultiLineString (directions, patterns, and trunk/branch segments stay separate members). */
        RouteLineGeometry: {
            /** @enum {string} */
            type: "MultiLineString";
            coordinates: number[][][];
        };
        RouteLineProperties: {
            /**
             * Format: uuid
             * @description Canonical route UUID
             */
            routeId: string;
            /** @description Line designation, e.g. 1, B, M */
            shortName: string;
            longName: string;
            /** @enum {string} */
            mode: "rail" | "subway" | "tram" | "bus" | "ferry" | "other";
            /** @description Hex color without leading '#', same convention as RouteRef.color — empty when the operator publishes none */
            color: string;
            agencyName: string;
            /**
             * @description 'shape' = real ingested path geometry (GTFS/OSM); 'stops' = straight polyline through ordered stops — schematic, not surveyed
             * @enum {string}
             */
            source: "shape" | "stops";
        };
        RouteLineFeature: {
            /** @enum {string} */
            type: "Feature";
            geometry: components["schemas"]["RouteLineGeometry"];
            properties: components["schemas"]["RouteLineProperties"];
        };
        /** @description GeoJSON FeatureCollection of route path geometries */
        RouteLineCollection: {
            /** @enum {string} */
            type: "FeatureCollection";
            features: components["schemas"]["RouteLineFeature"][];
        };
        /** @description Stop reference inside an itinerary. `id` is absent when the upstream station ref cannot be resolved to a canonical stop (external service). */
        JourneyStopRef: {
            /** Format: uuid */
            id?: string;
            name: string;
        };
        /** @description GeoJSON LineString sliced from an ingested route shape (GTFS) between the leg's endpoints. Absent when the route has no ingested shape that hugs the listed stops — clients then draw the stop-to-stop polyline. */
        LegShape: {
            /** @enum {string} */
            type: "LineString";
            coordinates: number[][];
        };
        /** @description A corridor that also serves the leg's endpoints. `stops` and `geometry` are derived from this alternative's own route_stops/shape rows — never borrowed from the provider's chosen leg. */
        JourneyLegAlternative: {
            /** Format: uuid */
            routeId: string;
            /** @description Provider line key, e.g. TJ:2 */
            line: string;
            /** @description Corridor/line code for chips — "2", "7F" */
            shortName?: string;
            /** @description Corridor long name, e.g. "Pulo Gadung - Monumen Nasional" */
            name?: string;
            operator?: string;
            /** @description Corridor's published color — hex without '#', same convention as RouteRef.color. Absent when the catalog stores none. */
            color?: string;
            /** @description Stop-to-stop hops across the served slice */
            stationCount?: number;
            /** @description Interior stops in ride order — leg endpoints excluded */
            stops: components["schemas"]["JourneyStopRef"][];
            geometry?: components["schemas"]["LegShape"];
        };
        JourneyLeg: {
            /** @description walk | ride */
            type: string;
            from: components["schemas"]["JourneyStopRef"];
            to: components["schemas"]["JourneyStopRef"];
            /** @description Walk distance in metres — walk legs only */
            distanceM?: number;
            /**
             * Format: date-time
             * @description Scheduled departure at the leg's origin (walk legs carry computed times)
             */
            depAt?: string;
            /**
             * Format: date-time
             * @description Scheduled arrival at the leg's destination
             */
            arrAt?: string;
            /**
             * Format: uuid
             * @description Canonical route UUID for ride legs
             */
            routeId?: string;
            /** @description Provider line key, e.g. MRTJ:M — diagnostic only */
            line?: string;
            operator?: string;
            /** @description Catalog mode vocabulary — rail | subway | tram | bus | ferry | other */
            mode?: string;
            /** @description Corridor's published color — hex without '#', same convention as RouteRef.color. Absent when the catalog stores none. */
            color?: string;
            stationCount?: number;
            /** @description Ordered stops ridden — real sequence from the schedule */
            stops?: components["schemas"]["JourneyStopRef"][];
            headsign?: string;
            /** @description True when the leg's times come from a frequency template or a reconstructed stop_time rather than a published trip time */
            estimated?: boolean;
            geometry?: components["schemas"]["LegShape"];
            /** @description Upcoming boardings at leg.from on this route — populated on the first ride leg only, computed from the same schedule snapshot. */
            nextDepartures?: components["schemas"]["Departure"][];
            /** @description Other catalog routes that also carry this leg's endpoints — corridors the rider can board instead of the planner's pick. */
            alternatives?: components["schemas"]["JourneyLegAlternative"][];
        };
        /** @description One end-to-end plan computed by the in-house schedule planner. Labels name the trade-off this option actually wins against the others — an honest differentiator, not a ranking score. */
        Itinerary: {
            /** @enum {string} */
            label: "fastest" | "fewest_transfers" | "least_walking" | "alternative";
            /** @description What this option trades off, e.g. "avoids KCI:C" */
            reason?: string;
            /** Format: date-time */
            departAt: string;
            /** Format: date-time */
            arriveAt: string;
            durationSec: number;
            legs: components["schemas"]["JourneyLeg"][];
            /** @description Count of walk legs — computed from legs */
            walkTransfers: number;
            rideLegs: number;
            /** @description rideLegs - 1 */
            transfers: number;
            /** @description Total walking distance across walk legs */
            walkM: number;
            /**
             * @description "scheduled" = every ride time is published; "estimated" = at least one leg rides a frequency template or reconstructed stop_time. Never live until realtime lands
             * @enum {string}
             */
            status: "scheduled" | "estimated";
        };
        FareSegment: {
            operator: string;
            from: components["schemas"]["JourneyStopRef"];
            to: components["schemas"]["JourneyStopRef"];
            /**
             * Format: int64
             * @description IDR integer — money is never a float
             */
            amount: number;
        };
        Fare: {
            /** @enum {string} */
            currency: "IDR";
            /** Format: int64 */
            total?: number;
            segments: components["schemas"]["FareSegment"][];
        };
        JourneyPlan: {
            from: components["schemas"]["JourneyStopRef"];
            to: components["schemas"]["JourneyStopRef"];
            /** @description The filters the plan was computed under — echoed back */
            query: {
                /**
                 * Format: date-time
                 * @description Departure anchor — set even for "leave now"
                 */
                departAt?: string;
                /** Format: date-time */
                arriveBy?: string;
                modes?: string[];
                maxWalkM?: number;
                maxTransfers?: number;
                stepFree?: boolean;
            };
            /** @description Computed plans, best first — empty when no plan exists for the filters. Exactly one carries label "fastest"; the rest earn trade-off labels or plain "alternative". */
            itineraries: components["schemas"]["Itinerary"][];
            /** @description Provider corridor fare estimate for the O-D pair — reference context, absent on upstream outage or when the pair is unknown upstream. */
            fareReference?: components["schemas"]["Fare"];
            source: {
                /** @description "schedule" — in-house planner over ingested data */
                provider: string;
                /** Format: date-time */
                requestedAt: string;
                /**
                 * Format: date-time
                 * @description When the planner's schedule snapshot was loaded
                 */
                snapshotAt: string;
            };
        };
        Session: {
            /** @description Opaque bearer token (256-bit, base64url). Shown exactly once — only its SHA-256 hash is stored server-side (ADR-010). */
            token: string;
            /** Format: date-time */
            expiresAt: string;
            /** Format: uuid */
            userId: string;
        };
        Visit: {
            /** Format: uuid */
            id: string;
            /** Format: uuid */
            stopId: string;
            /** Format: date-time */
            observedAt: string;
            /**
             * @description Derived server-side from evidence, never claimed by the client: `geofence` when a device fix was evaluated, `manual` when none was supplied, `trip` reserved for dwell/trip-context check-ins.
             * @enum {string}
             */
            validationMethod: "geofence" | "manual" | "trip";
            /**
             * @description `confirmed` = fix inside the geofence radius; `low_confidence` = outside it or no fix at all. Low-confidence visits are still recorded honestly (docs/42: mark, don't block).
             * @enum {string}
             */
            status: "confirmed" | "low_confidence";
            /**
             * Format: double
             * @description Meters between the fix and the stop (geofence check-ins only).
             */
            distanceM?: number;
            /** Format: date-time */
            createdAt: string;
        };
        VisitList: {
            visits: components["schemas"]["Visit"][];
        };
        CheckinRequest: {
            /** Format: uuid */
            stopId: string;
            /**
             * Format: uuid
             * @description Client-generated idempotency key (docs/24). Replays with the same key return the original row with `replayed: true`, never a duplicate.
             */
            clientMutationId: string;
            /**
             * Format: date-time
             * @description When the user says the visit happened (client clock).
             */
            observedAt: string;
            /**
             * Format: double
             * @description Device fix latitude. Must be sent together with `lon`; omitting both records a manual (unverified) check-in.
             */
            lat?: number;
            /** Format: double */
            lon?: number;
        };
        CheckinResponse: {
            visit: components["schemas"]["Visit"];
            /** @description true when the idempotency key matched an existing row. */
            replayed: boolean;
        };
        ProgressCount: {
            /** Format: int64 */
            visitedStops: number;
            /** Format: int64 */
            totalStops: number;
        };
        PassportProgress: components["schemas"]["ProgressCount"] & {
            byMode: (components["schemas"]["ProgressCount"] & {
                mode: string;
            })[];
            byRoute: (components["schemas"]["ProgressCount"] & {
                /** Format: uuid */
                routeId: string;
                /** @description Provider entity id of the corridor, e.g. "TJ:4B". */
                routeKey: string;
                name: string;
                mode: string;
                /** @description Corridor hex color (no leading */
                color?: string;
            })[];
            /** @description Progress inside each published curated collection (docs/42). */
            byCollection: (components["schemas"]["ProgressCount"] & {
                /** Format: uuid */
                collectionId: string;
                slug: string;
                title: string;
            })[];
        };
        JournalEntry: {
            /** Format: uuid */
            id: string;
            /** Format: uuid */
            stopId?: string;
            /** Format: uuid */
            visitEventId?: string;
            body: string;
            /**
             * @description Private by default — the only value the API accepts today.
             * @enum {string}
             */
            visibility: "private";
            /** Format: date-time */
            createdAt: string;
            /** Format: date-time */
            updatedAt: string;
        };
        JournalRequest: {
            /** Format: uuid */
            stopId?: string;
            /**
             * Format: uuid
             * @description Link to one of the caller's own visit events.
             */
            visitEventId?: string;
            body: string;
            /**
             * Format: date-time
             * @description PATCH only — optimistic version check against `updatedAt`.
             */
            baseUpdatedAt?: string;
        };
        CollectionSummary: {
            /** Format: uuid */
            id: string;
            slug: string;
            title: string;
            description?: string;
            /**
             * @description Editorial provenance (docs/13): 'curated' = editor-picked set; 'algorithmic' = computed set, labeled so it never poses as curation.
             * @enum {string}
             */
            kind: "curated" | "algorithmic";
            /**
             * Format: int64
             * @description Number of catalog stops in the set.
             */
            itemCount: number;
        };
        CollectionList: {
            collections: components["schemas"]["CollectionSummary"][];
        };
    };
    responses: never;
    parameters: never;
    requestBodies: never;
    headers: never;
    pathItems: never;
}
export type $defs = Record<string, never>;
export interface operations {
    getHealth: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Service is healthy */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        /** @enum {string} */
                        status: "ok";
                    };
                };
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
    getReady: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description All dependencies ready */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        /** @enum {string} */
                        status: "ok";
                    };
                };
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
    getVersion: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Build version */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        /** @example dev */
                        version: string;
                    };
                };
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
    listStations: {
        parameters: {
            query?: {
                query?: string;
                /**
                 * @description WGS84 viewport: minLon,minLat,maxLon,maxLat
                 * @example 106.70,-6.30,106.95,-6.10
                 */
                bbox?: string;
                limit?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Matching stations */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        stations: components["schemas"]["StationSummary"][];
                    };
                };
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
    getStation: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Station detail */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        station: components["schemas"]["StationDetail"];
                    };
                };
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
    getStationDepartures: {
        parameters: {
            query?: {
                /** @description Minutes ahead of "now" (Asia/Jakarta) to include. 1440 returns the whole service day. */
                window?: number;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Departures grouped per line and direction */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        departures: components["schemas"]["StationDepartures"];
                    };
                };
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
    listRoutes: {
        parameters: {
            query?: {
                query?: string;
                limit?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Matching routes */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        routes: components["schemas"]["RouteSummary"][];
                    };
                };
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
    getRoute: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Route detail */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        route: components["schemas"]["RouteDetail"];
                    };
                };
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
    listProviders: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Registered providers */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        providers: components["schemas"]["Provider"][];
                    };
                };
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
    listRouteLines: {
        parameters: {
            query: {
                /**
                 * @description WGS84 viewport: minLon,minLat,maxLon,maxLat
                 * @example 106.70,-6.30,106.95,-6.10
                 */
                bbox: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Route lines intersecting the viewport */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        lines: components["schemas"]["RouteLineCollection"];
                    };
                };
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
    planJourney: {
        parameters: {
            query: {
                /** @description Origin stop UUID */
                from: string;
                /** @description Destination stop UUID */
                to: string;
                /** @description Departure anchor ("leave at"). Mutually exclusive with `arriveBy`; absent both = leave now. */
                at?: string;
                /** @description Arrival bound ("be there by") — the planner maximizes the departure time instead. */
                arriveBy?: string;
                /**
                 * @description Comma-separated mode filter over the catalog vocabulary: rail, subway, tram, bus, ferry, other. Absent = all modes.
                 * @example rail,subway
                 */
                modes?: string;
                /** @description Per-transfer walking distance cap in metres. */
                maxWalkM?: number;
                /** @description Maximum number of ride transfers (rideLegs - 1). */
                maxTransfers?: number;
                /** @description Only stops with a published elevator amenity are usable as transfer or destination points. Accessibility coverage is partial — a filtered plan may be empty because metadata is incomplete, not because no step-free path exists. */
                stepFree?: boolean;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Computed itineraries (possibly empty) */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["JourneyPlan"];
                };
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
    createSession: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description New session */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Session"];
                };
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
    revokeSession: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Session revoked */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
    deleteAccount: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Account and all owned data deleted */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
    listVisits: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Own visit events */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["VisitList"];
                };
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
    checkin: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["CheckinRequest"];
            };
        };
        responses: {
            /** @description Idempotent replay — returns the already-recorded visit */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CheckinResponse"];
                };
            };
            /** @description Visit recorded */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CheckinResponse"];
                };
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
    getPassportProgress: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Progress counters */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["PassportProgress"];
                };
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
    listJournalEntries: {
        parameters: {
            query?: {
                limit?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Own entries */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        entries: components["schemas"]["JournalEntry"][];
                    };
                };
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
    createJournalEntry: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["JournalRequest"];
            };
        };
        responses: {
            /** @description Entry created */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        entry: components["schemas"]["JournalEntry"];
                    };
                };
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
    deleteJournalEntry: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Entry deleted */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
    updateJournalEntry: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["JournalRequest"];
            };
        };
        responses: {
            /** @description Entry updated */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        entry: components["schemas"]["JournalEntry"];
                    };
                };
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
    listCollections: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Published collections */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CollectionList"];
                };
            };
            /** @description Error envelope — shared by all endpoints */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["error"];
                };
            };
        };
    };
}
