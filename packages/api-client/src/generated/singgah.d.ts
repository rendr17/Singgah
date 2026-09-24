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
         * Search stations and stops
         * @description Case-insensitive search over display name, station code, and official name. An empty or missing `query` returns an empty list.
         */
        get: operations["searchStations"];
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
    searchStations: {
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
            /** @description Matching stations, relevance-ordered by the query index */
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
}
