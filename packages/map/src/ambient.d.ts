// Vite resolves CSS side-effect imports at build time; TS just needs to know
// the module exists.
declare module '*.css';
declare module '*.png' {
	const src: string;
	export default src;
}
