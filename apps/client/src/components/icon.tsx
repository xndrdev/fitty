import Svg, { Path } from 'react-native-svg';
import { Platform } from 'react-native';

const paths = {
  calendar: 'M5 5h14v15H5z M5 9h14 M8 3v4 M16 3v4 M8 13h2 M14 13h2 M8 16h2',
  history: 'M4 10a8 8 0 1 1 1 8 M4 4v6h6 M12 7v5l3 2',
  user: 'M16 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0 M4 21v-2a8 6 0 0 1 16 0v2',
  logout: 'M10 4H4v16h6 M10 12h11 M17 8l4 4-4 4',
  photo: 'M3 4h18v16H3z M3 16l6-6 5 5 3-3 4 4 M16 8h.01',
  camera: 'M3 7h5l2-3h4l2 3h5v13H3z M16 13a4 4 0 1 1-8 0 4 4 0 0 1 8 0',
  send: 'M12 20V4 M5 11l7-7 7 7',
  left: 'M14 6l-6 6 6 6',
  right: 'M10 6l6 6-6 6',
  close: 'M6 6l12 12 M18 6 6 18',
  plus: 'M12 5v14 M5 12h14',
  trash: 'M3 6h18 M9 6V3h6v3 M5 6l1 15h12l1-15 M10 10v7 M14 10v7',
  meal: 'M3 12h18 M4 12a8 8 0 0 0 16 0 M8 5v3 M12 3v5 M16 5v3',
  activity: 'M3 12h4l3-8 4 16 3-8h4',
  restaurant: 'M5 3v6a3 3 0 0 0 6 0V3 M8 3v18 M16 3v9h4 M20 3v18',
};
export type IconName = keyof typeof paths;

export function Icon({ name, color = '#2F5D3A', size = 18 }: { name: IconName; color?: string; size?: number }) {
  return <Svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke={color} strokeWidth={1.6} strokeLinecap="round" strokeLinejoin="round"
    {...(Platform.OS === 'web' ? { 'aria-hidden': true } : { accessibilityElementsHidden: true })}>
    <Path d={paths[name]} />
  </Svg>;
}
