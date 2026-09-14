import {useEffect, useState} from 'react';
export function Search({term}: {term: string}) {
  const [items, setItems] = useState<string[]>([]);
  useEffect(() => {
    fetch('/search?q=' + encodeURIComponent(term))
      .then(r => r.json()).then(setItems);
  }, [term]);
  return <ul>{items.map(item => <li key={item}>{item}</li>)}</ul>;
}
