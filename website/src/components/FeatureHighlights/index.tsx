import type {ReactNode} from 'react';
import Link from '@docusaurus/Link';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

type FeatureItem = {
  title: string;
  description: ReactNode;
  to: string;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'Two sources, one workflow',
    description:
      'vSphere and Nutanix AHV behave identically — same CLI, same jobs, same output layout. No provider-specific tribal knowledge required.',
    to: 'https://github.com/zyvorai/transiva#why-teams-start-here',
  },
  {
    title: 'Resumable jobs over REST',
    description:
      'transivad (compat: hypervisord) runs fleet exports as a daemon with a REST surface; transivactl (compat: hyperctl) drives list/export/info/submit.',
    to: 'https://github.com/zyvorai/transiva/blob/main/openapi.yaml',
  },
  {
    title: 'Offline exports, untouched source',
    description:
      'Conversion never mutates the live guest — artifacts are exported offline and the source VM stays untouched until cutover.',
    to: '/docs/nutanix',
  },
  {
    title: 'Scriptable end to end',
    description:
      'Interactive CLI for one-offs, daemon + REST for fleets. Legacy hyper* binary names stay as compatibility aliases for at least one major release.',
    to: 'https://github.com/zyvorai/transiva#binaries',
  },
  {
    title: 'Clean handoff into the Zyvor suite',
    description:
      'Export with Transiva → convert with h2kvm → assure with GuestKit → host on Machina → operate on Zeus OS. This repo covers discover and export.',
    to: 'https://github.com/zyvorai/h2kvm',
  },
  {
    title: 'Open by default',
    description:
      'Apache-2.0, no phone-home, no agent in the guest. Ships as source, Docker images, and RPM packages.',
    to: 'https://github.com/zyvorai/transiva/blob/main/LICENSE',
  },
];

function Feature({title, description, to}: FeatureItem) {
  return (
    <div className="col col--4">
      <Link to={to} className={styles.card}>
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
      </Link>
    </div>
  );
}

export default function FeatureHighlights(): ReactNode {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}
