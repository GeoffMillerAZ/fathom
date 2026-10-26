import React from 'react';
export class ClassComponent extends React.Component<{label: string}> {
  render() { return <span>{this.props.label}</span>; }
}
export function FunctionComponent(props: {label: string}) {
  return <span>{props.label}</span>;
}
export function Utility(value: {label: string}) {
  return value.label.toUpperCase();
}
function HiddenComponent(props: {label: string}) {
  return <span>{props.label}</span>;
}
