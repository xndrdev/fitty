import { useState } from 'react';
import { Platform, Pressable, Text, useWindowDimensions, View } from 'react-native';
import Svg, { Rect } from 'react-native-svg';
import { Entry, Summary } from '../lib/chat-api';
import { emptyTargets, targetProgress } from '../lib/targets';
import { styles as s } from '../styles/day-summary';
import { styles as editorStyles } from '../styles/entry-editor';
import { Button } from './ui';

const number = (value: number) => value.toLocaleString('de-DE', { maximumFractionDigits: 1 });
const sourceLabels = { estimate: 'Schätzung', user: 'Deine Angaben', device: 'Geräteangabe' };
const targetNumber = (value: number) => value.toLocaleString('de-DE', { maximumFractionDigits: 2 });

function TargetProgress({ label, actual, target, unit, color = '#2F5D3A' }: {
  label: string; actual: number; target: number; unit: string; color?: string;
}) {
  const { percent, remaining, above } = targetProgress(actual, target);
  const text = above > 0 ? `${targetNumber(above)} ${unit} über dem Ziel` : remaining > 0 ? `Noch ${targetNumber(remaining)} ${unit} bis zum Ziel` : 'Ziel erreicht';
  return <View style={s.progress}>
    <View accessible accessibilityRole="progressbar" accessibilityLabel={`${label}: Tagesziel`}
      aria-valuemin={0} aria-valuemax={100} aria-valuenow={percent}
      aria-valuetext={`${targetNumber(actual)} von ${targetNumber(target)} ${unit}. ${text}.`}>
      <Svg width="100%" height={6} {...(Platform.OS === 'web' ? { 'aria-hidden': true } : { accessibilityElementsHidden: true })}>
        <Rect width="100%" height={6} rx={3} fill="#EAECE1" />
        <Rect width={`${percent}%`} height={6} rx={3} fill={color} />
      </Svg>
    </View>
    <Text style={s.progressCaption}>{text}</Text>
  </View>;
}

type DaySummaryProps = {
  summary: Summary;
  onCorrect?: (entry: Entry) => void;
  onEdit?: (entry: Entry) => void;
  onDelete?: (entry: Entry) => void;
  onTargets?: () => void;
};

export function DaySummary({ summary, onCorrect, onEdit, onDelete, onTargets }: DaySummaryProps) {
  const [expanded, setExpanded] = useState(false);
  const [focused, setFocused] = useState(false);
  const { width } = useWindowDimensions();
  const compact = width < 520;
  const totals = summary.totals;
  const targets = summary.targets ?? emptyTargets();
  const hasTargets = Object.values(targets).some(value => value !== null);
  const hasActivity = summary.entries.some(entry => entry.kind === 'activity');
  const activityValues = [
    totals.duration_minutes > 0 ? `${number(totals.duration_minutes)} Min.` : null,
    totals.distance_km > 0 ? `${number(totals.distance_km)} km` : null,
    totals.activity_calories > 0 || totals.unknown_activity_calories === 0 ? `${number(totals.activity_calories)} kcal erfasst` : null,
  ].filter(Boolean).join(' · ');

  return <View style={s.card}>
    <View style={[s.overview, compact && s.overviewCompact, compact && hasTargets && s.targetsCompact]}>
      <View style={[s.calories, compact && s.caloriesCompact, hasTargets && s.caloriesWithTargets, compact && hasTargets && s.caloriesTargetCompact]}>
        <Text style={s.eyebrow}>Kalorien</Text>
        <Text style={[s.calorieValue, compact && s.calorieValueCompact]}>{number(totals.calories)}</Text>
        <Text style={s.caption}>{targets.calories === null ? 'kcal gegessen' : `von ${targetNumber(targets.calories)} kcal`}</Text>
        {targets.calories !== null && <TargetProgress label="Kalorien" actual={totals.calories} target={targets.calories} unit="kcal" />}
      </View>
      <View style={[s.macros, compact && hasTargets && s.macrosTargetCompact]}>
        {([
          { key: 'protein_g', label: 'Protein', dot: s.proteinDot, color: '#3D6A49' },
          { key: 'carbs_g', label: 'Kohlenhydrate', dot: s.carbsDot, color: '#AD9158' },
          { key: 'fat_g', label: 'Fett', dot: s.fatDot, color: '#B4765E' },
        ] as const).map((field, index) => {
          const target = targets[field.key];
          return <View key={field.key} style={[s.macroSection, index === 2 && s.lastMacroRow]}>
          <View style={s.macroRow}>
            <View style={s.macroLabel}><View style={[s.dot, field.dot]} /><Text style={s.label}>{field.label}</Text></View>
            <Text style={s.macroValue}>{number(totals[field.key])}{target === null ? '' : ` / ${targetNumber(target)}`} g</Text>
          </View>
          {target !== null && <TargetProgress label={field.label} actual={totals[field.key]} target={target} unit="g" color={field.color} />}
        </View>;
        })}
      </View>
    </View>
    {!!onTargets && <View style={[s.targetAction, compact && s.targetActionCompact]}>
      {!hasTargets && <Text style={s.caption}>Du kannst eigene Kalorien- und Makroziele ergänzen.</Text>}
      <Button variant="ghost" label={hasTargets ? 'Tagesziele bearbeiten' : 'Tagesziele festlegen'} onPress={onTargets} />
    </View>}

    {hasActivity && <View style={[s.activity, compact && s.activityCompact]}>
      <View style={s.activityHeading}>
        <View style={s.activityMark}><Text accessible={false} style={s.activityArrow}>↗</Text></View>
        <Text style={s.activityText}><Text style={s.activityLabel}>Bewegung</Text>{activityValues ? ` · ${activityValues}` : ''}</Text>
      </View>
      {totals.unknown_activity_calories > 0 && <Text style={s.caption}>{totals.unknown_activity_calories === 1 ? '1 Aktivität' : `${totals.unknown_activity_calories} Aktivitäten`} ohne Kalorienangabe.</Text>}
      <Text style={s.caption}>Aktivitätskalorien werden nicht vom Essen abgezogen.</Text>
    </View>}

    {summary.entries.length > 0 && <View style={[s.entries, compact && s.entriesCompact]}>
      <Pressable accessibilityRole="button"
        accessibilityLabel={expanded ? 'Einträge ausblenden' : `Einträge anzeigen (${summary.entries.length})`}
        accessibilityState={{ expanded }}
        onPress={() => setExpanded(value => !value)}
        onFocus={() => setFocused(true)} onBlur={() => setFocused(false)}
        style={({ pressed }) => [s.toggle, pressed && s.pressed, focused && s.focus]}>
        <Text style={s.toggleLabel}>Einträge ({summary.entries.length})</Text>
        <View style={s.toggleEnd}>
          {totals.estimated_entries > 0 && <Text style={s.estimateBadge}>{totals.estimated_entries} geschätzt</Text>}
          <Text accessible={false} style={s.chevron}>{expanded ? '−' : '+'}</Text>
        </View>
      </Pressable>
      {expanded && <View style={s.entryList}>
        {summary.entries.map(entry => <View key={entry.id} testID="tracking-entry" style={s.entry}>
          <View style={s.entryHeading}>
            <Text style={s.entryTitle}>{entry.label}</Text>
            <Text style={[s.sourceBadge, entry.source === 'estimate' && s.estimateBadge]}>{sourceLabels[entry.source]}</Text>
          </View>
          {!!entry.amount && <Text style={s.detail}>{entry.amount}</Text>}
          {entry.kind === 'food' ? <Text style={s.detail}>{[
            entry.calories === null ? 'Kalorien offen' : `${number(entry.calories)} kcal`,
            entry.protein_g === null ? 'Protein offen' : `${number(entry.protein_g)} g Protein`,
            entry.carbs_g === null ? 'Kohlenhydrate offen' : `${number(entry.carbs_g)} g KH`,
            entry.fat_g === null ? 'Fett offen' : `${number(entry.fat_g)} g Fett`,
          ].join(' · ')}</Text> : <Text style={s.detail}>{[
            entry.duration_minutes === null ? 'Dauer offen' : `${number(entry.duration_minutes)} Min.`,
            entry.distance_km === null ? 'Strecke offen' : `${number(entry.distance_km)} km`,
            entry.calories === null ? 'Kalorien offen' : `${number(entry.calories)} kcal`,
          ].join(' · ')}</Text>}
          {!!entry.notes && <Text style={s.detail}>{entry.notes}</Text>}
          <View style={editorStyles.actions}>
            <Button secondary label="Bearbeiten" accessibilityLabel={`${entry.label} bearbeiten`}
              disabled={!onEdit} onPress={() => onEdit?.(entry)} />
            <Button variant="ghost" label="Löschen" accessibilityLabel={`${entry.label} löschen`}
              disabled={!onDelete} onPress={() => onDelete?.(entry)} />
            <Button variant="ghost" label="Im Chat korrigieren" accessibilityLabel={`${entry.label} im Chat korrigieren`}
              disabled={!onCorrect} onPress={() => onCorrect?.(entry)} />
          </View>
        </View>)}
        {summary.pending_count > 0 && <Text style={s.caption}>Bearbeiten und Löschen sind nach der laufenden Auswertung wieder möglich.</Text>}
      </View>}
    </View>}
  </View>;
}
