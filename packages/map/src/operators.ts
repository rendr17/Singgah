import kciUrl from '../assets/operators/kci.png';
import lrtjUrl from '../assets/operators/lrtj.png';
import lrtjbdbUrl from '../assets/operators/lrtjbdb.png';
import mrtjUrl from '../assets/operators/mrtj.png';
import tjUrl from '../assets/operators/tj.png';

// Official operator marks (sources/licenses: docs/35_DATA_SOURCES.md), keyed by
// the provider-declared operator code carried on station metadata. TransitMap
// registers each image under the `op-*` name returned by operatorIcon; codes
// not listed here keep the generic marker — never guess a mark.
export const OPERATOR_ICON_URLS: Record<string, string> = {
	TJ: tjUrl,
	KCI: kciUrl,
	MRTJ: mrtjUrl,
	LRTJ: lrtjUrl,
	LRTJBDB: lrtjbdbUrl
};

/** MapLibre image name for an operator code, or '' when it has no mark. */
export function operatorIcon(operator: string | undefined | null): string {
	return operator && operator in OPERATOR_ICON_URLS ? `op-${operator.toLowerCase()}` : '';
}
