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
         * @description One canonical route with the stops it serves. Stops are sorted by name — ordered trip/line-shape data lands with a later ingest.
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
    "/api/v1/journeys": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Station-to-station itinerary
         * @description One provider-computed itinerary between two canonical station UUIDs. Legs come back normalized to canonical stop/route UUIDs; fares are IDR integers. `itinerary` is null when the upstream source finds no plan — never an error. Always `status: "scheduled"`; no realtime exists yet.
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
        RouteSummary: components["schemas"]["RouteRef"] & {
            providerCode: string;
        };
        RouteDetail: components["schemas"]["RouteSummary"] & {
            /** @description Served stops sorted by name — authoritative stop ordering arrives with line/trip ingest, so this list is not a sequence diagram yet. */
            stops: components["schemas"]["StopRef"][];
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
        /** @description Stop reference inside an itinerary. `id` is absent when the upstream station ref cannot be resolved to a canonical stop (external service). */
        JourneyStopRef: {
            /** Format: uuid */
            id?: string;
            name: string;
        };
        JourneyLeg: {
            /** @description walk | ride | raw upstream type for unrecognised legs */
            type: string;
            from: components["schemas"]["JourneyStopRef"];
            to: components["schemas"]["JourneyStopRef"];
            distanceM?: number;
            /**
             * Format: uuid
             * @description Canonical route UUID for ride legs
             */
            routeId?: string;
            /** @description Provider line key, e.g. MRTJ:M — diagnostic only */
            line?: string;
            operator?: string;
            stationCount?: number;
            /** @description Ordered stops ridden — real sequence from the provider */
            stops?: components["schemas"]["JourneyStopRef"][];
            headsign?: string;
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
        Itinerary: {
            legs: components["schemas"]["JourneyLeg"][];
            /** @description Count of walk legs — computed from legs, not the provider's transferCount */
            walkTransfers: number;
            rideLegs: number;
            fare?: components["schemas"]["Fare"];
            totalDistanceM: number;
            /**
             * @description Static provider data — never live until realtime lands
             * @enum {string}
             */
            status: "scheduled";
        };
        JourneyPlan: {
            from: components["schemas"]["JourneyStopRef"];
            to: components["schemas"]["JourneyStopRef"];
            /** @description null when the provider computes no plan for the pair */
            itinerary: components["schemas"]["Itinerary"] | null;
            source: {
                provider: string;
                /** Format: date-time */
                requestedAt: string;
            };
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
    planJourney: {
        parameters: {
            query: {
                /** @description Origin stop UUID */
                from: string;
                /** @description Destination stop UUID */
                to: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Normalized plan (itinerary may be null) */
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
}
